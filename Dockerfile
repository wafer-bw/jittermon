# build binary
FROM --platform=$BUILDPLATFORM golang:1.27.3 AS builder
ARG TARGETOS TARGETARCH
WORKDIR /jittermon
COPY . .
RUN go mod download \
    && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-w -s -extldflags '-static'" -a -o /go/bin/main .

# build image
FROM scratch
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/main /go/bin/main
WORKDIR /go/bin
ENTRYPOINT ["./main"]
