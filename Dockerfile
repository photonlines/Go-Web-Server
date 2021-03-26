FROM golang:1.14


WORKDIR /go/


ADD . .


EXPOSE 8888

RUN go build src/main.go

CMD ./main
