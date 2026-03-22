# jittermon
[![codecov](https://codecov.io/gh/wafer-bw/jittermon/graph/badge.svg?token=EZfdMqKD7p)](https://codecov.io/gh/wafer-bw/jittermon)
[![checks](https://github.com/wafer-bw/jittermon/actions/workflows/checks.yml/badge.svg)](https://github.com/wafer-bw/jittermon/actions/workflows/checks.yml)
![Example Screenshot](./.media/examplescreen.png)

## Getting Started
First, review [fly.toml](./fly.toml), you may want to update
`primary_region = 'yyz'` to your own [region](https://fly.io/docs/reference/regions).

```sh
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
# set remote send address
# make sure to replace YOURIP with the ipv4 address of your local machine
# make sure to configure port forwarding on your local network
fly secrets set JITTERMON_JITTER_SEND_ADDRS=YOURIP:8081
```
```sh
# create & update .env file with appropriate address
# make sure to replace FLYADDRESS with the ipv4 you allocated above
echo JITTERMON_JITTER_SEND_ADDRS=FLYADDRESS:8080 > .env
# build docker image
docker build -t jittermon .
# start jittermon locally
docker compose up -d
# observe metrics at http://localhost:3000/d/aec2tnhcwbuo0b
```
```sh
# stop
docker compose down
```

## TODOs
- handle src/dst id/address confusion.
- back off send rate when failing.
- handle timeouts that take longer than interval to avoid misreporting packet
  loss.
- at least one alternative to fly.io for running remote peer.
- resolve TODOs in code.
