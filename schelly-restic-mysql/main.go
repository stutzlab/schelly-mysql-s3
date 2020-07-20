package main

import (
	"flag"
	"fmt"
	"os"
	"time"
	"net/http"
	"context"
	"bytes"
	"path"	

	"github.com/sirupsen/logrus"
	"github.com/flaviostutz/schelly-webhook"
	
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"database/sql"
	"github.com/go-sql-driver/mysql"
	"github.com/jamf/go-mysqldump"		
)

var sourcePath *string
var repoDir *string

//ResticBackuper sample backuper
type ResticBackuper struct {
}

func main() {
	logrus.Info("====Starting Restic REST server====")
	resticBackuper := ResticBackuper{}
	err := schellyhook.Initialize(resticBackuper)
	if err != nil {
		logrus.Errorf("Error initializating Schellyhook. err=%s", err)
		os.Exit(1)
	}
}

//RegisterFlags register command line flags
func (sb ResticBackuper) RegisterFlags() error {
	sourcePath = flag.String("source-path", "/backup-source", "Backup source path")
	repoDir = flag.String("repo-dir", "/backup-repo", "Restic repository of backups")
	return nil
}

//Init initialize
func (sb ResticBackuper) Init() error {
	return nil
}

//CreateNewBackup creates a new backup
func (sb ResticBackuper) CreateNewBackup(apiID string, timeout time.Duration, shellContext *schellyhook.ShellContext) error {
	logrus.Infof("CreateNewBackup() apiID=%s timeout=%d s", apiID, timeout.Seconds)
	Mysqldump(apiID)
	return nil
}

//GetAllBackups returns all backups from underlaying backuper. optional for Schelly
func (sb ResticBackuper) GetAllBackups() ([]schellyhook.SchellyResponse, error) {
	logrus.Debugf("GetAllBackups")
	DownloadAll()
	return nil, nil
}

//GetBackup get an specific backup along with status
func (sb ResticBackuper) GetBackup(apiID string) (*schellyhook.SchellyResponse, error) {
	logrus.Debugf("GetBackup apiID=%s", apiID)
	Download(apiID)
	return nil, nil
}

//DeleteBackup removes current backup from underlaying backup storage
func (sb ResticBackuper) DeleteBackup(apiID string) error {
	logrus.Debugf("DeleteBackup apiID=%s", apiID)
	Delete(apiID)
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------

var (
	S3_BUCKET = os.Getenv("S3_BUCKET") 
	S3_PATH = os.Getenv("S3_PATH")
	DUMP_CONNECTION_NAME = os.Getenv("DUMP_CONNECTION_NAME")
	DUMP_CONNECTION_HOST = os.Getenv("DUMP_CONNECTION_HOST")
	DUMP_CONNECTION_AUTH_USERNAME = os.Getenv("DUMP_CONNECTION_AUTH_USERNAME")
	DUMP_CONNECTION_AUTH_PASSWORD = os.Getenv("DUMP_CONNECTION_AUTH_PASSWORD")	
	AWS_ACCESS_KEY_ID = os.Getenv("AWS_ACCESS_KEY_ID")	
	AWS_SECRET_ACCESS_KEY = os.Getenv("AWS_SECRET_ACCESS_KEY")			
)
var sess = connectAWS()
func connectAWS() *session.Session {
	sessRegion := session.Must(session.NewSession(&aws.Config{
		LogLevel: aws.LogLevel(aws.LogDebugWithHTTPBody),
		Credentials: credentials.NewStaticCredentials(AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, "default"),
	}))
	region, err := s3manager.GetBucketRegion(context.Background(), sessRegion, S3_BUCKET, "us-west-1")
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, "default"),
	})
	if err != nil {
		panic(err)
	}	
	return sess
}

func ClearDir(dir string) error {
	dirRead, err := os.Open(dir)
	if err != nil {
		return err
	}
	dirFiles, err := dirRead.Readdir(0)
	if err != nil {
		return err
	}
	for index := range dirFiles {
		entery := dirFiles[index]
		filename := entery.Name()
		fullPath := path.Join(dir, filename)
		os.Remove(fullPath)
	}
	return nil
}

// ----------------------------------------------------------------------------------------------------

func Mysqldump(apiID string){
	// Open connection to database
	config := mysql.NewConfig()
	config.User = DUMP_CONNECTION_AUTH_USERNAME
	config.Passwd = DUMP_CONNECTION_AUTH_PASSWORD
	config.DBName = DUMP_CONNECTION_NAME
	config.Net = "tcp"
	config.Addr = DUMP_CONNECTION_HOST

	dumpDir := S3_PATH	
	
	dumpFilenameFormat := apiID //fmt.Sprintf("%s-20060102T150405", config.DBName) // accepts time layout string and add .sql at the end of file

    err := os.Remove(dumpDir)

	if err := os.MkdirAll(dumpDir, 0755); err != nil {
		logrus.Errorf("Error mkdir:", err)
		return
	}

	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		logrus.Errorf("Error opening database:", err)
		return
	}

	// Register database with mysqldump
	dumper, err := mysqldump.Register(db, dumpDir, dumpFilenameFormat)
	if err != nil {
		logrus.Errorf("Error registering databse:", err)
		return
	}

	// Dump database to file
	if err := dumper.Dump(); err != nil {
		logrus.Errorf("Error dumping:", err)
		return
	}
	
	if file, ok := dumper.Out.(*os.File); ok {
		fmt.Printf("Successfully mysqldump...")
		UploadS3(file.Name(), apiID)
		err := ClearDir(dumpDir)
		if err!=nil{
			logrus.Errorf("Error ClearDir:", err)
		}		
	} else {
		logrus.Errorf("It's not part of *os.File, but dump is done")
	}
	// Close dumper, connected database and file stream.
	dumper.Close()	
}
func UploadS3(s3Key_Mysql string, apiID string){
	file, err := os.Open(s3Key_Mysql)
	if err != nil {
		logrus.Errorf("File not opened> %q\n", err)
		return
	}

    // Get file size and read the file content into a buffer
    fileInfo, _ := file.Stat()
    var size int64 = fileInfo.Size()
    buffer := make([]byte, size)
    file.Read(buffer)

	svc := s3.New(sess)
    _, err = svc.PutObject(&s3.PutObjectInput{
        Bucket:               aws.String(S3_BUCKET),
        Key:                  aws.String(s3Key_Mysql),
        Body:                 bytes.NewReader(buffer),
        ContentLength:        aws.Int64(size),
        ContentType:          aws.String(http.DetectContentType(buffer)),
        ContentDisposition:   aws.String("attachment"),
        //ServerSideEncryption: aws.String("AES256"),
    })
	if err != nil {
		logrus.Errorf("Something went wrong uploading the file> %q\n", err)
		return
	}	
	logrus.Infof("Successfully uploaded to %s\n", S3_BUCKET)

    return	
}

// ----------------------------------------------------------------------------------------------------

func ListAll() []*s3.Object {
	svc := s3.New(sess)
	resp, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(S3_BUCKET)})
	if err != nil {
		logrus.Errorf("Unable to list items in bucket > ", err)
	}
	logrus.Infof("List> ", resp.Contents)
	
	// for _, item := range resp.Contents {
		// fmt.Println("Name:         ", *item.Key)
		// fmt.Println("Last modified:", *item.LastModified)
		// fmt.Println("Size:         ", *item.Size)
		// fmt.Println("Storage class:", *item.StorageClass)
		// fmt.Println("")
	// }	
	
	
	logrus.Infof("S3_BUCKET> ", S3_BUCKET)
	logrus.Infof("S3_PATH> ", S3_PATH)
	logrus.Infof("DUMP_CONNECTION_NAME> ", DUMP_CONNECTION_NAME)
	logrus.Infof("DUMP_CONNECTION_HOST> ", DUMP_CONNECTION_HOST)
	logrus.Infof("DUMP_CONNECTION_AUTH_USERNAME> ", DUMP_CONNECTION_AUTH_USERNAME)
	logrus.Infof("DUMP_CONNECTION_AUTH_PASSWORD> ", DUMP_CONNECTION_AUTH_PASSWORD)
	logrus.Infof("AWS_ACCESS_KEY_ID> ", AWS_ACCESS_KEY_ID)
	logrus.Infof("AWS_SECRET_ACCESS_KEY> ", AWS_SECRET_ACCESS_KEY)

	return resp.Contents
}

// ----------------------------------------------------------------------------------------------------

func DownloadAll(){
	Download("")
}
func Download(apiID string){
	downloader := s3manager.NewDownloader(sess)
	if apiID=="" {
		listAll := 	ListAll()
		for _, item := range listAll {  
			filename := *item.Key
			file, err := os.Create(filename)
			if err != nil {
				logrus.Errorf("Unable to open file > %q\n", err)
			}
			defer file.Close()		
		
			_, err = downloader.Download(file, &s3.GetObjectInput{
				Bucket: aws.String(S3_BUCKET),
				Key:    aws.String(filename),
			})
			if err != nil {
				logrus.Errorf("Something went wrong retrieving the file from S3> %q\n", err)
				return
			}				
		}		
	} else {
		file, err := os.Create(apiID)
		if err != nil {
			logrus.Errorf("Unable to open file > %q\n", err)
		}
		defer file.Close()	
	
		_, err = downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(S3_BUCKET),
			Key:    aws.String(apiID),
		})
		if err != nil {
			logrus.Errorf("Something went wrong retrieving the file from S3 > %q\n", err)
			return
		}
	}
	logrus.Infof("Downloaded")
	return			
}

// ----------------------------------------------------------------------------------------------------

func DeleteAll(){
	Delete("")
}
func Delete(apiID string){

	svc := s3.New(sess)
	
	if apiID=="" {
		iter := s3manager.NewDeleteListIterator(svc, &s3.ListObjectsInput{
			Bucket: aws.String(S3_BUCKET),
		})
		if err := s3manager.NewBatchDeleteWithClient(svc).Delete(aws.BackgroundContext(), iter); err != nil {
			logrus.Errorf("Unable to delete objects > %q\n", err)
		}
	
	} else {
		var err error	
		_, err = svc.DeleteObject(&s3.DeleteObjectInput{Bucket: aws.String(S3_BUCKET), Key: aws.String(apiID)})
		if err != nil {
			logrus.Errorf("Unable to delete object > %q\n", err)
		}
		err = svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
			Bucket: aws.String(S3_BUCKET),
			Key:    aws.String(apiID),
		})
		if err != nil {
			logrus.Errorf("Unable to delete objects > %q\n", err)
			return
		}
	}
	
	logrus.Infof("Deleted object(s) from bucket: %s\n", S3_BUCKET)
	return
}

