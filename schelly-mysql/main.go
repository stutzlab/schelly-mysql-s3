package main

import (
	"net/http"
	"os"
	"strings"	
	"bytes"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	
	"github.com/sirupsen/logrus"
	
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"github.com/jamf/go-mysqldump"	
	
	"github.com/flaviostutz/schelly-webhook"
	"time"
)

// ----------------------------------------------------------------------------------------------------

type MySQLBackuper struct{}

func (sb MySQLBackuper) CreateNewBackup(apiID string, timeout time.Duration, shellContext *schellyhook.ShellContext) error {
			// Remove the server files 
			e := ClearDir(S3_PATH)
			if e != nil { 
				logrus.Errorf("Could not remove the server files: %q", e) 
			} 
			return Mysqldump(apiID);
}

func (sb MySQLBackuper) GetAllBackups() ([]schellyhook.SchellyResponse, error) {
			resp, err := List("");
			if err != nil {
				return nil, err
			}	
			if len(resp) == 0 {
				return nil, nil
			}				
			return resp, nil			
}

func (sb MySQLBackuper) GetBackup(apiID string) (*schellyhook.SchellyResponse, error) {
			resp, err := List(getKey(apiID));
			if err != nil {
				return nil, err
			}
			if len(resp) == 0 {
				return nil, nil
			}			
			return &schellyhook.SchellyResponse{
				ID:      resp[0].ID,
				DataID:  resp[0].DataID,
				Status:  resp[0].Status,
				Message: resp[0].Message,
				SizeMB:  resp[0].SizeMB,
			}, nil
}

func (sb MySQLBackuper) DeleteBackup(apiID string) error {
			return Delete(getKey(apiID));
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
func lastString(ss []string) string {
    return ss[len(ss)-1]
}
func getKey(t string) string {
    return S3_PATH + "/" + t + ".sql"
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

func Mysqldump(S3_Key string) error{
	// Open connection to database
	config := mysql.NewConfig()
	config.User = DUMP_CONNECTION_AUTH_USERNAME
	config.Passwd = DUMP_CONNECTION_AUTH_PASSWORD
	config.DBName = DUMP_CONNECTION_NAME
	config.Net = "tcp"
	config.Addr = DUMP_CONNECTION_HOST
    err := os.Remove(S3_PATH)
	if err := os.MkdirAll(S3_PATH, 0755); err != nil {
		logrus.Errorf("Error mkdir: %s", err)
		return err
	}
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		logrus.Errorf("Error opening database: %s", err)
		return err
	}
	// Register database with mysqldump
	dumper, err := mysqldump.Register(db, S3_PATH, S3_Key)

	if err != nil {
		logrus.Errorf("Error registering databse: %s", err)
		return err
	}
	// Dump database to file
	if err := dumper.Dump(); err != nil {
		logrus.Errorf("Error dumping: %s", err)
		return err
	}
	if file, ok := dumper.Out.(*os.File); ok {
		logrus.Infof("Successfully mysqldump...")
		return UploadS3(file.Name())		
	} else {
		logrus.Errorf("It's not part of *os.File, but dump is done")
	}
	// Close dumper, connected database and file stream.
	dumper.Close()	
	return nil
}

// -----------------------------------------------------------------

func UploadS3(S3_Key string) error{
	file, err := os.Open(S3_Key)
	if err != nil {
		logrus.Errorf("File not opened: %q", err)
		return err
	}
    // Get file size and read the file content into a buffer
    fileInfo, _ := file.Stat()
    var size int64 = fileInfo.Size()
    buffer := make([]byte, size)
    file.Read(buffer)
	svc := s3.New(sess)
    _, err = svc.PutObject(&s3.PutObjectInput{
        Bucket:               aws.String(S3_BUCKET),
        Key:                  aws.String(S3_Key),
        Body:                 bytes.NewReader(buffer),
        ContentLength:        aws.Int64(size),
        ContentType:          aws.String(http.DetectContentType(buffer)),
        ContentDisposition:   aws.String("attachment"),
        //ServerSideEncryption: aws.String("AES256"),
    })
	if err != nil {
		logrus.Errorf("Something went wrong uploading the file: %q", err)
		return err
	}	
	logrus.Infof("Successfully uploaded to %s", S3_BUCKET)
	file.Close()
    return nil
}

// ----------------------------------------------------------------------------------------------------

func List(S3_Key string) ([]schellyhook.SchellyResponse, error) {
	svc := s3.New(sess)
	resp, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(S3_BUCKET)})
	if err != nil {
		logrus.Errorf("Unable to list items in bucket: %s", err)
		return nil, err
	}
	backups := make([]schellyhook.SchellyResponse, 0)
	for _, item := range resp.Contents {
		S3key := *item.Key
		S3Size := *item.Size
		S3Msg := *item.StorageClass
		
		if len(S3_Key) == 0 {
			sr := schellyhook.SchellyResponse{
						ID:      lastString(strings.Split(S3key, "/")),
						DataID:  S3key,
						Status:  "available",
						Message: S3Msg,
						SizeMB:  float64(S3Size),
			}
			backups = append(backups, sr)
		}
		if strings.Compare(S3_Key, *item.Key) == 0 {
			sr := schellyhook.SchellyResponse{
						ID:      lastString(strings.Split(S3key, "/")),
						DataID:  S3key,
						Status:  "available",
						Message: S3Msg,
						SizeMB:  float64(S3Size),
			}
			backups = append(backups, sr)
		}
	}    	
	return backups, nil
}

// ----------------------------------------------------------------------------------------------------

func Delete(S3_Key string) error {
	logrus.Infof("S3_Key: %s", S3_Key)
	svc := s3.New(sess)	
	if len(S3_Key) == 0 {
		logrus.Errorf("Unable to delete without 'key'")
	} else {
		var err error	
		_, err = svc.DeleteObject(&s3.DeleteObjectInput{Bucket: aws.String(S3_BUCKET), Key: aws.String(S3_Key)})
		if err != nil {
			logrus.Errorf("Unable to delete object: %q", err)
			return err
		}
		err = svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
			Bucket: aws.String(S3_BUCKET),
			Key:    aws.String(S3_Key),
		})
		if err != nil {
			logrus.Errorf("Unable to delete object: %q", err)
			return err
		}
	}
	logrus.Infof("Deleted object from bucket: %s", S3_BUCKET)
	return nil
}