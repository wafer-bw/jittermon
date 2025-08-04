# jittermon
[![codecov](https://codecov.io/gh/wafer-bw/jittermon/graph/badge.svg?token=EZfdMqKD7p)](https://codecov.io/gh/wafer-bw/jittermon)
[![checks](https://github.com/wafer-bw/jittermon/actions/workflows/checks.yml/badge.svg)](https://github.com/wafer-bw/jittermon/actions/workflows/checks.yml)

## Demos
Preconfigured demos using Docker, Grafana, Prometheus, & Loki
![Example Screenshot](./.media/examplescreen.png)

### Local P2P
Measures traffic between local peers on the same network.
```sh
# build docker image
docker build -t jittermon .
# start the demo
docker compose -f demo/docker-compose-local-p2p.yml up -d
# observe metrics at http://localhost:3000/d/aec2tnhcwbuo0b
```
```sh
# stop
docker compose -f demo/docker-compose-local-p2p.yml down
```

### Remote P2P
Measures traffic between a local & remote peer using fly.io. This is for
advanced users only. Will cost at least $2/mo on fly.io because we need a
non-shared ipv4 address. For some reason gRPC isn't well-supported by Fly on
shared IP addresses.

First, review [fly.toml](./fly.toml), you may want to update
`primary_region = 'yyz'` to your own [region](https://fly.io/docs/reference/regions).

```sh
# build docker image
docker build -t jittermon .
# deploy to fly...
#   when executing the above follow these choices for the prompts:
#   ? Would you like to copy its configuration to the new app?
#     Yes
#   ? Do you want to tweak these settings before proceeding?
#     No
#   ? Create .dockerignore from 1 .gitignore files?
#     No
#   ? Would you like to allocate dedicated ipv4 and ipv6 addresses now?
#     No
fly launch
# scale fly app down to one machine, we don't want or need multiple
fly scale count 1
# allocate a dedicated ipv4 address
fly ips allocate-v4
# create & update .env file with appropriate address
# make sure to replace FLYADDRESS with the ipv4 you allocated above
echo JITTERMON_P2P_LATENCY_SEND_ADDRS=FLYADDRESS:PORT > .env
echo JITTERMON_TRACE_SEND_ADDRS=FLYADDRESS:PORT >> .env
# start the demo
docker compose -f demo/docker-compose-remote-p2p.yml up -d
# observe metrics at http://localhost:3000/d/aec2tnhcwbuo0b
```
```sh
# stop
docker compose -f demo/docker-compose-remote-p2p.yml down
```

## TODOs
- rename Poll to Sample. on samplers.
- split samplers into package per type.
- cleanup package names then cleanup type names.
- simplify recorder interface by using interface assertion to:
  - determine sample type.
  - determine labels.
  - determine timestamp.
  - no longer need `Sample` or `SampleType`
- log recorder.
- add a way to request samplers by name and/or redefine config as more
  structured.
- update readme with better getting started guide.
- handle src/dst id/address confusion.
- back off send rate when failing.
- handle timeouts that take longer than interval to avoid misreporting packet
  loss.
- route tracing.
  - hop filtering in grafana.
  - more useful visualizations.
- promote required packages out of internal.
- at least one alternative to fly.io for demo.
