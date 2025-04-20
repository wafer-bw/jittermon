# build binary
FROM golang:1.24.2 AS builder
WORKDIR /jittermon
COPY . .
RUN go get -v ./... \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s -extldflags '-static'" -a -o /go/bin/main .

# build image
# TODO: use scratch once traceroute has pure go implementation.
FROM alpine:3.21.3
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/main /go/bin/main
WORKDIR /go/bin
ENTRYPOINT ["./main"]
