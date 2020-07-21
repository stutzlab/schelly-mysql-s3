FROM golang:1.10 AS BUILD

#install external dependencies first
ADD /main.go $GOPATH/src/schelly-mysql/main.go
RUN go get -v schelly-mysql

#now build source code
ADD schelly-mysql $GOPATH/src/schelly-mysql
RUN go get -v schelly-mysql


FROM ubuntu:18.04

RUN apt-get update
RUN apt-get install -y ca-certificates

EXPOSE 7070

ENV LOG_LEVEL 'debug'

ENV S3_PATH /mysql
ENV S3_BUCKET bem-backups-dev
ENV S3_REGION us-west-1
ENV DUMP_CONNECTION_NAME bem_saude
ENV DUMP_CONNECTION_HOST docker.for.win.localhost:3306
ENV DUMP_CONNECTION_AUTH_USERNAME bem_saude
ENV DUMP_CONNECTION_AUTH_PASSWORD bem_saude	
ENV AWS_ACCESS_KEY_ID aaaaaaaaaa
ENV AWS_SECRET_ACCESS_KEY aaaaaaaaaa

COPY --from=BUILD /go/bin/* /bin/
ADD startup.sh /

CMD [ "/startup.sh" ]
