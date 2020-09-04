# schelly-mysql

Backup MySQL

See more about Schelly at http://github.com/flaviostutz/schelly

# Usage

docker-compose .yml

```
version: '3.5'

services:

  schelly:
    image: flaviostutz/schelly
    ports:
      - 8080:8080
    environment:
      - LOG_LEVEL=debug
      - BACKUP_NAME=schelly-pgdump
      - WEBHOOK_URL=http://localhost:7070/backups
      - BACKUP_CRON_STRING=0 */1 * * * *
      - RETENTION_MINUTELY=5
      - WEBHOOK_GRACE_TIME=20

  mysql-api:
    build: .
    ports:
      - 7070:7070
    environment:
      - LOG_LEVEL=debug
      - S3_PATH=mysql
      - S3_BUCKET=bucket
      - S3_REGION=us-west-1
      - DUMP_CONNECTION_NAME=name
      - DUMP_CONNECTION_HOST=server:port
      - DUMP_CONNECTION_AUTH_USERNAME=user
      - DUMP_CONNECTION_AUTH_PASSWORD=pass
      - AWS_ACCESS_KEY_ID=key
      - AWS_SECRET_ACCESS_KEY=sec
```

* execute ```docker-compose up``` and see logs

* run:

```
// #create a new backup
// curl POST localhost:7070/backups

// #list all backups
// curl localhost:7070/backups

// #list existing backup
// curl localhost:7070/backups/abc123

// #remove existing backup
// curl DELETE localhost:7070/backups/abc123	
```

