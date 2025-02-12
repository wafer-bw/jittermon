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
# build docker image
docker build -t jittermon .
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
- handle timezones
- loki log ingestion in demo?
- Handle all possible I/O outside of req/resp in a separate go routine reading
  from a channel?
- long term nice to have
  - checks workflow only runs when go code changes
  - Use ICMP for RTT?
  - Look into establishing streaming connections to avoid TCP?
  - Cobra CLI
- add tracing
