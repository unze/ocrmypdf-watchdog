FROM golang:alpine as builder
RUN apk update && apk add --no-cache git
COPY . $GOPATH/src/ocrmypdf-watchdog/
WORKDIR $GOPATH/src/ocrmypdf-watchdog/
ENV GO111MODULE=off
RUN go get -d -v
FROM jbarlow83/ocrmypdf:v11.7.0
FROM jbarlow83/ocrmypdf:v11.7.0
COPY --from=builder /go/bin/main /app/
WORKDIR /app
VOLUME /in /bak /out
ENTRYPOINT ["/app/main"]
