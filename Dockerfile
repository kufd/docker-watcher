FROM golang:1.7.1

COPY ./ /go/src/docker-watcher
RUN cd /go/src/docker-watcher && go get -d -v && go install -v

ENTRYPOINT docker-watcher