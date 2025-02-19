# build binary
FROM golang:1.24.0 AS builder
RUN apt-get update && apt-get install -yq \
    apt-transport-https \
    build-essential \
    ca-certificates \
    libssl-dev
WORKDIR /jittermon
COPY . .

RUN go get -v ./... \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=true -ldflags="-w -s -extldflags '-static'" -a -o /go/bin/main .

# build image
# TODO: use scratch once traceroute has pure go implementation
FROM alpine:latest
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/main /go/bin/main
WORKDIR /go/bin
ENTRYPOINT ["./main"]
