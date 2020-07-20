# schelly-restic-mysql

This exposes the common functions of Restic with Schelly REST APIs so that it can be used as a backup backend for Schelly (https://github.com/flaviostutz/schelly#webhook-spec)

See more at http://github.com/flaviostutz/schelly

# Usage

docker-compose .yml

```
version: '3.5'

services:

  restic-mysql-api:
    build: .
    ports:
      - 7070:7070
    environment:
      - RESTIC_PASSWORD=123
      - LOG_LEVEL=debug
      - PRE_BACKUP_COMMAND=dd if=/dev/zero of=/backup-source/TESTFILE bs=100MB count=2
      - POST_BACKUP_COMMAND=rm /backup-source/TESTFILE
      - SOURCE_DATA_PATH=/backup-source/TESTFILE
      - TARGET_DATA_PATH=/backup-repo
      - S3_PATH=/backups/mysql
      - S3_BUCKET='bem-backups-dev'
      - DUMP_CONNECTION_NAME='bem_saude'
      - DUMP_CONNECTION_HOST='localhost:3306'
      - DUMP_CONNECTION_AUTH_USERNAME='bem_saude'
      - DUMP_CONNECTION_AUTH_PASSWORD='bem_saude'
      - AWS_ACCESS_KEY_ID='xxx'
      - AWS_SECRET_ACCESS_KEY='xxx'	  
```

* execute ```docker-compose up``` and see logs

* run:

```
#create a new backup
curl -X POST localhost:7070/backups

#list existing backups
curl -X localhost:7070/backups

#get info about an specific backup
curl localhost:7070/backups/abc123

#remove existing backup
curl -X DELETE localhost:7070/backups/abc123

```

# REST Endpoints

As in https://github.com/flaviostutz/schelly#webhook-spec
