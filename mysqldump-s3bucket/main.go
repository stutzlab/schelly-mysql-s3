
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"	
	"context"
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
)

// ----------------------------------------------------------------------------------------------------

const (
	S3_REGION = "us-west-1"
	
	// ENV ???
	
	myCredentials = "C:\\Users\\ricar\\.aws\\credentials" 
	
	S3_BUCKET = "bem-backups-dev" 
    S3_PATH = "/backups/mysql"
    DUMP_CONNECTION_NAME = "bem_saude"
    DUMP_CONNECTION_HOST = "localhost"
    DUMP_CONNECTION_PORT = "3306"
    DUMP_CONNECTION_AUTH_USERNAME = "bem_saude"
    DUMP_CONNECTION_AUTH_PASSWORD = "bem_saude"	
)
var sess = connectAWS()
func connectAWS() *session.Session {
	sessRegion := session.Must(session.NewSession(&aws.Config{
		LogLevel: aws.LogLevel(aws.LogDebugWithHTTPBody),
		Credentials: credentials.NewSharedCredentials(myCredentials, "default"),
	}))
	region, err := s3manager.GetBucketRegion(context.Background(), sessRegion, S3_BUCKET, S3_REGION)
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewSharedCredentials(myCredentials, "default"),
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

func main() {

	http.HandleFunc("/get/", handlerDownload)  
	http.HandleFunc("/list/", handlerList)  
	http.HandleFunc("/delete/", handlerDelete) 
	http.HandleFunc("/upload/", handler_Mysqldump_UploadS3) 
	
	log.Fatal(http.ListenAndServe(":7070", nil))
}

// ----------------------------------------------------------------------------------------------------
// ----------------------------------------------------------------------------------------------------

func handler_Mysqldump_UploadS3(w http.ResponseWriter, r *http.Request) {
	Mysqldump();
}


func Mysqldump(){
	// Open connection to database
	config := mysql.NewConfig()
	config.User = DUMP_CONNECTION_AUTH_USERNAME
	config.Passwd = DUMP_CONNECTION_AUTH_PASSWORD
	config.DBName = DUMP_CONNECTION_NAME
	config.Net = "tcp"
	config.Addr = DUMP_CONNECTION_HOST+":"+DUMP_CONNECTION_PORT

	dumpDir := S3_PATH	
	
	dumpFilenameFormat := fmt.Sprintf("%s-20060102T150405", config.DBName) // accepts time layout string and add .sql at the end of file

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
		UploadS3(file.Name())
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

// -----------------------------------------------------------------

func UploadS3(s3Key_Mysql string){
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

func handlerList(w http.ResponseWriter, r *http.Request) {
	ListAll()
	return 
}


func ListAll() []*s3.Object {
	svc := s3.New(sess)
	resp, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(S3_BUCKET)})
	if err != nil {
		logrus.Errorf("Unable to list items in bucket > ", err)
	}
	logrus.Infof("List> ", resp.Contents)
	return resp.Contents
}

// ----------------------------------------------------------------------------------------------------

func handlerDownload(w http.ResponseWriter, r *http.Request) {
	Download(strings.Replace(r.URL.Path, "/get/", "", 1));
}


func DownloadAll(){
	Download("")
}
func Download(filename string){
	downloader := s3manager.NewDownloader(sess)
	if filename=="" {
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
			logrus.Errorf("Something went wrong retrieving the file from S3 > %q\n", err)
			return
		}
	}
	logrus.Infof("Downloaded")
	return	

	// for _, item := range resp.Contents {
		// fmt.Println("Name:         ", *item.Key)
		// fmt.Println("Last modified:", *item.LastModified)
		// fmt.Println("Size:         ", *item.Size)
		// fmt.Println("Storage class:", *item.StorageClass)
		// fmt.Println("")
	// }		
}

// ----------------------------------------------------------------------------------------------------

func handlerDelete(w http.ResponseWriter, r *http.Request) {
	Delete(strings.Replace(r.URL.Path, "/delete/", "", 1));
}


func DeleteAll(){
	Delete("")
}
func Delete(filename string){

	logrus.Infof("Filename: %s\n", filename)
	
	svc := s3.New(sess)
	
	if filename=="" {
		iter := s3manager.NewDeleteListIterator(svc, &s3.ListObjectsInput{
			Bucket: aws.String(S3_BUCKET),
		})
		if err := s3manager.NewBatchDeleteWithClient(svc).Delete(aws.BackgroundContext(), iter); err != nil {
			logrus.Errorf("Unable to delete objects > %q\n", err)
		}
	
	} else {
		var err error	
		_, err = svc.DeleteObject(&s3.DeleteObjectInput{Bucket: aws.String(S3_BUCKET), Key: aws.String(filename)})
		if err != nil {
			logrus.Errorf("Unable to delete object > %q\n", err)
		}
		err = svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
			Bucket: aws.String(S3_BUCKET),
			Key:    aws.String(filename),
		})
		if err != nil {
			logrus.Errorf("Unable to delete objects > %q\n", err)
			return
		}
	}
	
	logrus.Infof("Deleted object(s) from bucket: %s\n", S3_BUCKET)
	return
}


