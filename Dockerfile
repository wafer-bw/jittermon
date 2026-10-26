# build binary
FROM --platform=$BUILDPLATFORM golang:1.27.3 AS builder
ARG TARGETOS TARGETARCH TARGETVARIANT
WORKDIR /jittermon
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags="-w -s -extldflags '-static'" -a -o /go/bin/main .

# build image
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/bin/main /go/bin/main
ENTRYPOINT ["/go/bin/main"]
