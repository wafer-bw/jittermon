# jittermon
[![codecov](https://codecov.io/gh/wafer-bw/jittermon/graph/badge.svg?token=EZfdMqKD7p)](https://codecov.io/gh/wafer-bw/jittermon)
[![checks](https://github.com/wafer-bw/jittermon/actions/workflows/checks.yml/badge.svg)](https://github.com/wafer-bw/jittermon/actions/workflows/checks.yml)
![Example Screenshot](./.media/examplescreen.png)

Jittermon is a network monitoring tool specifically focused on ping & jitter.
It can be run standalone, providing monitoring of packet loss, ping, and jitter
or with a remote peer allowing independent monitoring of upstream and downstream
jitter. It uses [Loki](https://grafana.com/oss/loki/), [Grafana](https://grafana.com/),
and [Prometheus](https://prometheus.io/) for storing and visualizing data.

## Author's Notes
Unfortunately, I don't have any plans to maintain this project in any serious
capacity aside from yearly [dependabot](https://docs.github.com/en/code-security/tutorials/secure-your-dependencies/dependabot-quickstart-guide) PRs. If you would like to take this project
further, please feel free to fork or copy it.

Follow the getting started section below to begin. Advanced readers may also
want to check out the [contributing](./.github/CONTRIBUTING.md) file for more
technical details.

<details>
<summary>Background Story (click to expand)</summary>

A year ago I was experiencing strange internet issues that were hard to identify
in a meaningful way to my ISP. The best information I had was that I was seeing
elevated packet loss, ping, & jitter, and that during such events on a VoIP call
I could hear others but they couldn't hear me. At the time I was also using
[PingPlotter](https://www.pingplotter.com/) (which heavily inspired this project
and I recommend you check out) but it couldn't explain the issue fully.

Oddly enough, while playing [Deadlock](https://store.steampowered.com/app/1422450/Deadlock/)
with their network graphs enabled I noticed something peculiar about my issue
that PingPlotter wasn't able to see: it was primarily my upstream connection
that was experiencing problems. So I set out to measure and observe my upstream
and downstream jitter similar to the Deadlock network graphs by creating this
tool.

I spent a lot of time going back and forth on the complexity of the application
and its design, flip-flopping on topics such as an extensible library, in-house
visualization tools, executable binary, and other implementation details.
However, I have to move on from the project and wanted it in a usable open
source state so I tightened things up to just this very simple tool you run in
Docker.
</details>

## Getting Started
1. Clone the repo or download and extract the source code from the latest
   version [here](https://github.com/wafer-bw/jittermon/releases).
2. Ensure you have the following requirements installed:
  - [Docker](https://www.docker.com/get-started/)
  - [Docker Compose](https://docs.docker.com/compose/)
3.  Follow either the [Standalone](#standalone) (simple, free) or
    [With Remote Peer](#with-remote-peer) (advanced, may incur costs) sections below
    to get started.

### Standalone
1. Build docker image.
    ```sh
    docker build -t jittermon .
    ```
2. Run the app.
    ```sh
    docker compose up -d
    ```
3. View metrics in your browser at http://localhost:3000/d/aec2tnhcwbuo0b
   (username: `admin`, password: `demo`).
4. Stop the app.
    ```sh
    docker compose down
    ```

### With Remote Peer
For our remote peer example, we use [fly.io](https://fly.io/) because it is
cheap and easy. Before proceeding, review their pricing details [here](https://fly.io/pricing/)
to make sure you're comfortable with any costs you may incur. When this project
started they had free allowances but that doesn't seem to be the case anymore as
stated [here](https://fly.io/docs/about/pricing/#legacy-free-allowances).

1. Review [fly.toml](./fly.toml), you'll want to update `primary_region` to your
   own [region](https://fly.io/docs/reference/regions).
2. Deploy to fly
    ```sh
    fly launch
    # when executing the above follow these choices for the prompts:
    # ? Would you like to copy its configuration to the new app?
    #   Yes
    # ? Do you want to tweak these settings before proceeding?
    #   No
    # ? Create .dockerignore from 1 .gitignore files?
    #   No
    # ? Would you like to allocate dedicated ipv4 and ipv6 addresses now?
    #   Yes
    ```
3. Scale the deployment down to one machine. We don't need or want multiple.
    ```sh
    fly scale count 1
    ```
4. Set the remote peer's send address to your IP address (make sure you've port
   forwarded, and replace `YOURIPHERE` with your public IPv4 address).
    ```sh
    fly secrets set JITTERMON_PTP_SEND_ADDRS=YOURIPHERE:8081
    ```
5. Save the IP address of your fly app from step 2 above to `.env` (replace
   `FLYADDRESS` with the actual IPv4 address).
    ```sh
    echo JITTERMON_PTP_SEND_ADDRS=FLYADDRESS:8080 > .env
    ```
6. Build docker image.
    ```sh
    docker build -t jittermon .
    ```
7. Run the app.
    ```sh
    docker compose up -d
    ```
8. View metrics in your browser at http://localhost:3000/d/aec2tnhcwbuo0b
   (use username `admin` and password `demo` to log in).
9. Stop the app.
    ```sh
    docker compose down
    ```

## TODOs
- Better example screenshot that isn't downscaled but still renders okay in
  GitHub.
- Follow standalone getting started on windows.
- Have non-technical user try to follow standalone getting started.
