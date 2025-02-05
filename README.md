# jittermon

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
- config defines which metrics to collect
- Organize folders, nesting stuff like fly and docker out of root.
- Overhead of gRPC vs ICMP seems to be like 20ms, likely want to add a ms trim
  flag to trim out the protocol overhead. More reading [here](https://bbengfort.github.io/2016/11/ping-vs-grpc/).
- Add ICMP RTT.
- Handle all possible I/O outside of req/resp in a separate go routine reading
  from a channel.
- Look into establishing streaming connections to avoid TCP?
- It looks like there is two different streams of RTT/Jitter levels, why is it
  like that?
- Cobra CLI
