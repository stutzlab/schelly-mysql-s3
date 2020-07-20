FROM golang:1.10 AS BUILD

#install external dependencies first
ADD /main.go $GOPATH/src/schelly-restic-mysql/main.go
RUN go get -v schelly-restic-mysql

#now build source code
ADD schelly-restic-mysql $GOPATH/src/schelly-restic-mysql
RUN go get -v schelly-restic-mysql


FROM ubuntu:18.04

RUN apt-get update
RUN apt-get install -y restic

VOLUME [ "/backup-source" ]
VOLUME [ "/backup-repo" ]

EXPOSE 7070

ENV RESTIC_PASSWORD ''
ENV LISTEN_PORT 7070
ENV LISTEN_IP '0.0.0.0'
ENV LOG_LEVEL 'debug'

ENV PRE_POST_TIMEOUT '7200'
ENV PRE_BACKUP_COMMAND ''
ENV POST_BACKUP_COMMAND ''
ENV SOURCE_DATA_PATH '/backup-source'
ENV TARGET_DATA_PATH '/backup-repo'

ENV S3_PATH '/backups/mysql'
ENV S3_BUCKET 'bem-backups-dev'
ENV DUMP_CONNECTION_NAME 'bem_saude'
ENV DUMP_CONNECTION_HOST 'localhost:3306'
ENV DUMP_CONNECTION_AUTH_USERNAME 'bem_saude'
ENV DUMP_CONNECTION_AUTH_PASSWORD 'bem_saude'	
ENV AWS_ACCESS_KEY_ID 'xxx'
ENV AWS_SECRET_ACCESS_KEY 'xxx'

COPY --from=BUILD /go/bin/* /bin/
ADD startup.sh /

CMD [ "/startup.sh" ]
