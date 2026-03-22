# build binary
FROM golang:1.26.1 AS builder
WORKDIR /jittermon
COPY . .
RUN go get -v ./... \
    && CGO_ENABLED=0 go build -ldflags="-w -s -extldflags '-static'" -a -o /go/bin/main .

# build image
FROM scratch
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/main /go/bin/main
WORKDIR /go/bin
ENTRYPOINT ["./main"]
