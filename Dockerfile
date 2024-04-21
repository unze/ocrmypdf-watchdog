FROM golang:alpine as builder
RUN apk update && apk add --no-cache git
COPY . $GOPATH/src/ocrmypdf-watchdog/
WORKDIR $GOPATH/src/ocrmypdf-watchdog/
ENV GO111MODULE=on
RUN go mod init
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /go/bin/main .
FROM jbarlow83/ocrmypdf-alpine:v16.2.0
COPY --from=builder /go/bin/main /app/
WORKDIR /app
VOLUME /in /bak /out
ENTRYPOINT ["/app/main"]
