# jittermon
[![codecov](https://codecov.io/gh/wafer-bw/jittermon/graph/badge.svg?token=EZfdMqKD7p)](https://codecov.io/gh/wafer-bw/jittermon)
[![checks](https://github.com/wafer-bw/jittermon/actions/workflows/checks.yml/badge.svg)](https://github.com/wafer-bw/jittermon/actions/workflows/checks.yml)

![Example Screenshot](./.media/examplescreen.png)

```sh
# build docker image
docker build -t jittermon .
# start the demo
docker compose -f demo/docker-compose-local.yml up -d
# http://localhost:3000/d/aec2tnhcwbuo0b
docker compose -f demo/docker-compose-local.yml down
```

```sh
# deploy to fly
fly deploy
# start the demo
docker compose -f demo/docker-compose-fly.yml up -d
# http://localhost:3000/d/aec2tnhcwbuo0b
docker compose -f demo/docker-compose-fly.yml down
```

## Notes
- Won't work in fly.io with a shared IPv4, you will need a dedicated one which
  costs $2/mo.

## TODOs
- use http transcoding as server for p2platency if in http mode?
- use http client for p2p latency if in http mode?
- decide how to split samplers
  - one package of base types? in main tie it all together with config?
  - split package per type?
- simplify recorder interface by using interface assertion to
  - determine sample type
  - determine labels
  - determine timestamp
  - no longer need `Sample` or `SampleType`
- add contextual log handler for common attributes
- move logger into context
- add a way to request samplers by name
- handle src/dst id/address confusion
- back off send rate when failing
- route tracing
  - hop filtering in grafana
  - more useful visualizations
- persist loki data locally
- promote samplers out of internal
- long term nice to have
- make checks workflow only run when go code changes
- at least one alternative to fly.io for demo
- dedicated RTT sampler
  - likely best to use UDP, other options would be ICMP/TCP(DNS)
- Look into establishing streaming connections for p2p to avoid TCP overhead?
- Cobra CLI for main.go execution
