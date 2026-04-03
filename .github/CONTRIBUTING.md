# Contributing Guide
Brief contributing guide to help anyone looking to do more with Jittermon than
is covered in the [README](../README.md).

## Requirements
- [go](https://go.dev/doc/install)
- [docker](https://www.docker.com/get-started/)
- [docker compose](https://docs.docker.com/compose/)
- [buf](https://buf.build/docs/cli/installation/)
- [flyctl](https://fly.io/docs/flyctl/install/)

## Usage
Aside from what's already described in the readme's [Getting Started](../README.md#getting-started)
you can use the following scripts.
```sh
./scripts/fmt      # format Go code and protos.
./scripts/generate # generate Go mocks and proto stubs.
./scripts/lint     # lint Go code and protos.
./scripts/test     # test Go code and see current coverage (ignores generated files).
```

## Peer to Peer
Peer to peer communication is established using gRPC. The client and server
implementation are [here](../internal/grpcptp/grpcptp.go) and the protos are
[here](../internal/proto). You can regenerate the stubs from the protos with
`buf generate` or `./scripts/generate`.

Emits ping, packet loss, upstream jitter, and downstream jitter metrics.

You can provide multiple peer addresses as a comma separated list in the
`JITTERMON_PTP_SEND_ADDRS` env var.

## Peer to External
Peer to external address communication is performed over UDP. The client
implementation is [here](../internal/udpptx/udpptx.go). UDP is used instead of
ICMP so that we don't need admin/root access to the client device.

Emits ping, jitter, and packet loss metrics.

You can provide multiple external addresses as a comma separated list in the
`JITTERMON_PTX_SEND_ADDRS` env var.

## Jitter Calculation
Jitter is calculated using the implementation [here](../internal/jitter/jitter.go)
which is designed to follow [RFC 3550 Section 6.4.1](https://datatracker.ietf.org/doc/html/rfc3550#section-6.4.1).

## Metrics
Metrics are implemented and maintained [here](../internal/otel/otel.go). The
dashboard can be updated by saving its JSON representation [here](../app/grafana/dashboards/default.json)
which Grafana will prompt you to do if you attempt to edit & save it in your
browser.

The metrics emitted by this app are:
- `sent.packets`
- `lost.packets`
- `ping`
- `jitter`
- `upstream.jitter`
- `downstream.jitter`

Each metric will also have a `src` label for the source (ID via the 
`JITTERMON_ID` env var) and `dst` label for the destination (address via the
`_ADDRS` env vars).

All metrics emitted by this app as mentioned above are also sent to stdout at
the DEBUG log level when `JITTERMON_LOG_LEVEL` is `DEBUG` or lower. However, the
dashboard log panels filter them out. You can query them [here](http://localhost:3000/explore?schemaVersion=1&panes=%7B%2211e%22:%7B%22datasource%22:%22P8E80F9AEF21F6940%22,%22queries%22:%5B%7B%22refId%22:%22A%22,%22expr%22:%22%7Bcompose_project%3D%5C%22jittermon%5C%22,compose_service%3D%5C%22jittermon%5C%22%7D%22,%22queryType%22:%22range%22,%22datasource%22:%7B%22type%22:%22loki%22,%22uid%22:%22P8E80F9AEF21F6940%22%7D,%22editorMode%22:%22code%22,%22direction%22:%22backward%22%7D%5D,%22range%22:%7B%22from%22:%22now-1h%22,%22to%22:%22now%22%7D,%22panelsState%22:%7B%22logs%22:%7B%22visualisationType%22:%22logs%22%7D%7D%7D%7D&orgId=1).

## Potential Next Steps
As mentioned in the [author's notes](../README.md#authors-notes) I don't plan to
take this further but I'm dropping some ideas here for anyone that may be
interested.

- An ICMP alternative to UDP which will be better for measuring ping.
- A trace route implementation & metrics.
- Connecting to remote peer over IPv6 would save the flat $2/mo cost of the IPv4
  address.
- Cheaper remote peer alternative.
