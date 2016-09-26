FROM golang:1.7.1-alpine

COPY ./ /go/src/docker-watcher
RUN apk update && \
    apk upgrade && \
    apk add git && \
    cd /go/src/docker-watcher && \
    go get -d -v && \
    go install -v && \
    apk del git

ENTRYPOINT ["docker-watcher"]