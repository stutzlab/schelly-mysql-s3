
package main

import (
	"fmt"
	"log"
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
)

// ----------------------------------------------------------------------------------------------------

func main() {  
	http.HandleFunc("/backups", router) 
	http.HandleFunc("/backups/", router)  
	log.Fatal(http.ListenAndServe(":7070", nil))	
}
func router(w http.ResponseWriter, r *http.Request) {
	S3_Key_MySQL := ""
	if strings.Compare(r.URL.Path, "/backups") == 0 { 
		S3_Key_MySQL = strings.Replace(r.URL.Path, "/backups", "", 1)	
	} else {
		S3_Key_MySQL = strings.Replace(r.URL.Path, "/backups/", "", 1)	
	}
    switch r.Method {
		case "GET":
			List(S3_Key_MySQL);	
		case "POST":
			Mysqldump();
		case "DELETE":
			Delete(S3_Key_MySQL);	
		case "COPY":
			Download(S3_Key_MySQL);		
		default:
			fmt.Fprintf(w, "Sorry, only GET, POST, DELETE & DOWNLOAD methods are supported.")
    }
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

func Mysqldump(){
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
		return
	}
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		logrus.Errorf("Error opening database: %s", err)
		return
	}
	// Register database with mysqldump
	dumper, err := mysqldump.Register(db, S3_PATH, dumpFilenameFormat)
	if err != nil {
		logrus.Errorf("Error registering databse: %s", err)
		return
	}
	// Dump database to file
	if err := dumper.Dump(); err != nil {
		logrus.Errorf("Error dumping: %s", err)
		return
	}
	if file, ok := dumper.Out.(*os.File); ok {
		logrus.Infof("Successfully mysqldump...")
		UploadS3(file.Name())
		err := ClearDir(S3_PATH)
		if err!=nil{
			logrus.Errorf("Error ClearDir: %s", err)
		}		
	} else {
		logrus.Errorf("It's not part of *os.File, but dump is done")
	}
	// Close dumper, connected database and file stream.
	dumper.Close()	
}

// -----------------------------------------------------------------

func UploadS3(S3_Key_MySQL string){
	file, err := os.Open(S3_Key_MySQL)
	if err != nil {
		logrus.Errorf("File not opened: %q", err)
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
        Key:                  aws.String(S3_Key_MySQL),
        Body:                 bytes.NewReader(buffer),
        ContentLength:        aws.Int64(size),
        ContentType:          aws.String(http.DetectContentType(buffer)),
        ContentDisposition:   aws.String("attachment"),
        //ServerSideEncryption: aws.String("AES256"),
    })
	if err != nil {
		logrus.Errorf("Something went wrong uploading the file: %q", err)
		return
	}	
	logrus.Infof("Successfully uploaded to %s", S3_BUCKET)
    return	
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

func DeleteAll(){
	Delete("")
	return
}
func Delete(S3_Key_MySQL string){
	logrus.Infof("S3_Key_MySQL: %s", S3_Key_MySQL)
	svc := s3.New(sess)	
	if len(S3_Key_MySQL) == 0 {
		iter := s3manager.NewDeleteListIterator(svc, &s3.ListObjectsInput{
			Bucket: aws.String(S3_BUCKET),
		})		
		if err := s3manager.NewBatchDeleteWithClient(svc).Delete(aws.BackgroundContext(), iter); err != nil {
			logrus.Errorf("Unable to delete objects: %q", err)
		}
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
			logrus.Errorf("Unable to delete objects: %q", err)
			return
		}
	}
	logrus.Infof("Deleted object(s) from bucket: %s", S3_BUCKET)
	return
}