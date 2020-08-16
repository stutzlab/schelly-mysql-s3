package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"	
	"bytes"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	
	"github.com/sirupsen/logrus"
	
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"github.com/jamf/go-mysqldump"	
	
	"github.com/flaviostutz/schelly-webhook/schellyhook"
	"time"
)

// ----------------------------------------------------------------------------------------------------

type MySQLBackuper struct{}

func (sb MySQLBackuper) CreateNewBackup(apiID string, timeout time.Duration, shellContext *schellyhook.ShellContext) error {
			// resp := Mysqldump();
			// backup := schellyhook.SchellyResponse{
				// ID:      "0",
				// Status:  "error",
				// Message:  "",
			// }	
			// if len(resp) != 0 {
				// backup = schellyhook.SchellyResponse{
					// ID:      resp,
					// Status:  "available",
					// Message:  "",
				// }				
			// }
			
			return nil
}

func (sb MySQLBackuper) GetAllBackups() ([]schellyhook.SchellyResponse, error) {
			S3_Key_MySQL := ""
			resp := List(S3_Key_MySQL);
			if len(resp) == 0 {
				return nil, nil
			}			
			backups := make([]schellyhook.SchellyResponse, 0)
			for _, item := range resp {
				S3key := *item.Key
				S3Size := *item.Size
				S3Msg := *item.StorageClass
				sr := schellyhook.SchellyResponse{
					ID:      S3key,
					DataID:  S3key,
					Status:  "available",
					Message: S3Msg,
					SizeMB:  float64(S3Size),
				}
				backups = append(backups, sr)
			}
			return backups, nil			
}

func (sb MySQLBackuper) GetBackup(apiID string) (*schellyhook.SchellyResponse, error) {
			S3_Key_MySQL := apiID
			resp := List(S3_Key_MySQL);
			if len(resp) == 0 {
				return nil, nil
			}		
			S3key := *resp[0].Key
			S3Size := *resp[0].Size
			S3Msg := *resp[0].StorageClass
			return &schellyhook.SchellyResponse{
				ID:      S3key,
				DataID:  S3key,
				Status:  "available",
				Message: S3Msg,
				SizeMB:  float64(S3Size),
			}, nil
}

func (sb MySQLBackuper) DeleteBackup(apiID string) error {
			Delete(apiID);
			return nil
}

func main() {
	logrus.Info("====Starting server====")
	mySQLBackuper := MySQLBackuper{}
	err := schellyhook.Initialize(mySQLBackuper)
	if err != nil {
		logrus.Errorf("Error initializating Schellyhook. err=%s", err)
		os.Exit(1)
	}
}

func (sb MySQLBackuper) Init() error {
	return nil
}
func (sb MySQLBackuper) RegisterFlags() error {
	return nil
}

// ----------------------------------------------------------------------------------------------------

var (
	S3_REGION = os.Getenv("S3_REGION")
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
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(S3_REGION),
		Credentials: credentials.NewStaticCredentials(AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, ""),
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

func Mysqldump() string{
	// Open connection to database
	config := mysql.NewConfig()
	config.User = DUMP_CONNECTION_AUTH_USERNAME
	config.Passwd = DUMP_CONNECTION_AUTH_PASSWORD
	config.DBName = DUMP_CONNECTION_NAME
	config.Net = "tcp"
	config.Addr = DUMP_CONNECTION_HOST
	dumpFilenameFormat := fmt.Sprintf("%s-20060102T150405", config.DBName) // accepts time layout string and add .sql at the end of file
    err := os.Remove(S3_PATH)
	if err := os.MkdirAll(S3_PATH, 0755); err != nil {
		logrus.Errorf("Error mkdir: %s", err)
		return ""
	}
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		logrus.Errorf("Error opening database: %s", err)
		return ""
	}
	// Register database with mysqldump
	dumper, err := mysqldump.Register(db, S3_PATH, dumpFilenameFormat)
	if err != nil {
		logrus.Errorf("Error registering databse: %s", err)
		return ""
	}
	// Dump database to file
	if err := dumper.Dump(); err != nil {
		logrus.Errorf("Error dumping: %s", err)
		return ""
	}
	if file, ok := dumper.Out.(*os.File); ok {
		logrus.Infof("Successfully mysqldump...")
		return UploadS3(file.Name())		
	} else {
		logrus.Errorf("It's not part of *os.File, but dump is done")
	}
	// Close dumper, connected database and file stream.
	dumper.Close()	
	return ""
}

// -----------------------------------------------------------------

func UploadS3(S3_Key_MySQL string) string{
	file, err := os.Open(S3_Key_MySQL)
	if err != nil {
		logrus.Errorf("File not opened: %q", err)
		return ""
	}
    // Get file size and read the file content into a buffer
    fileInfo, _ := file.Stat()
    var size int64 = fileInfo.Size()
    buffer := make([]byte, size)
    file.Read(buffer)
	svc := s3.New(sess)
    _, err = svc.PutObject(&s3.PutObjectInput{
        Bucket:               aws.String(S3_BUCKET),
        Key:                  aws.String(S3_Key_MySQL),
        Body:                 bytes.NewReader(buffer),
        ContentLength:        aws.Int64(size),
        ContentType:          aws.String(http.DetectContentType(buffer)),
        ContentDisposition:   aws.String("attachment"),
        //ServerSideEncryption: aws.String("AES256"),
    })
	if err != nil {
		logrus.Errorf("Something went wrong uploading the file: %q", err)
		return ""
	}	
	logrus.Infof("Successfully uploaded to %s", S3_BUCKET)
    return S3_Key_MySQL
}

// ----------------------------------------------------------------------------------------------------

func ListAll() []*s3.Object {
	return List("")
}
func List(S3_Key_MySQL string) []*s3.Object {
	svc := s3.New(sess)
	resp, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(S3_BUCKET)})
	if err != nil {
		logrus.Errorf("Unable to list items in bucket: %s", err)
	}
	i := 1
	fmt.Println("-------------------  Start List ----------------------")
	for _, item := range resp.Contents {
		if strings.Compare(S3_Key_MySQL, *item.Key) == 0 { 
			fmt.Println("S3_Key:       ", *item.Key)
			fmt.Println("Last modified:", *item.LastModified)
			fmt.Println("Size:         ", *item.Size)
			fmt.Println("Storage class:", *item.StorageClass)
			fmt.Println("")	
		} 
		if len(S3_Key_MySQL) == 0 {
			fmt.Println("", i)
			fmt.Println("S3_Key:       ", *item.Key)
			fmt.Println("Last modified:", *item.LastModified)
			fmt.Println("Size:         ", *item.Size)
			fmt.Println("Storage class:", *item.StorageClass)
			fmt.Println("")			
		} 
		i++
	}	    
	fmt.Println("-------------------   End List  ----------------------")
	return resp.Contents
}

// ----------------------------------------------------------------------------------------------------

func DownloadAll(){
	Download("")
	return
}
func Download(S3_Key_MySQL string){
	downloader := s3manager.NewDownloader(sess)
	if len(S3_Key_MySQL) == 0 {
		listAll := 	ListAll()
		for _, item := range listAll {  
			S3_Key_MySQL := *item.Key
			file, err := os.Create(S3_Key_MySQL)
			if err != nil {
				logrus.Errorf("Unable to open file: %q", err)
			}
			defer file.Close()		
		
			_, err = downloader.Download(file, &s3.GetObjectInput{
				Bucket: aws.String(S3_BUCKET),
				Key:    aws.String(S3_Key_MySQL),
			})
			if err != nil {
				logrus.Errorf("Something went wrong retrieving the file from S3> %q", err)
				return
			}				
		}		
	} else {
		file, err := os.Create(S3_Key_MySQL)
		if err != nil {
			logrus.Errorf("Unable to open file: %q", err)
		}
		defer file.Close()	
	
		_, err = downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(S3_BUCKET),
			Key:    aws.String(S3_Key_MySQL),
		})
		if err != nil {
			logrus.Errorf("Something went wrong retrieving the file from S3: %q", err)
			return
		}
	}
	logrus.Infof("Downloaded")
	return			
}

// ----------------------------------------------------------------------------------------------------

func Delete(S3_Key_MySQL string){
	logrus.Infof("S3_Key_MySQL: %s", S3_Key_MySQL)
	svc := s3.New(sess)	
	if len(S3_Key_MySQL) == 0 {
		logrus.Errorf("Unable to delete without 'key'")
	} else {
		var err error	
		_, err = svc.DeleteObject(&s3.DeleteObjectInput{Bucket: aws.String(S3_BUCKET), Key: aws.String(S3_Key_MySQL)})
		if err != nil {
			logrus.Errorf("Unable to delete object: %q", err)
		}
		err = svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
			Bucket: aws.String(S3_BUCKET),
			Key:    aws.String(S3_Key_MySQL),
		})
		if err != nil {
			logrus.Errorf("Unable to delete object: %q", err)
			return
		}
	}
	logrus.Infof("Deleted object from bucket: %s", S3_BUCKET)
	return
}