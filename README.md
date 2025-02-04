# jittermon

## Demo
This demo will run two jittermon peers within Docker that talk to eachother. It
uses a Grafana dashboard to visualize metrics.

Follow these steps to see a demo of jittermon in action:
1. Install [Docker](https://docs.docker.com/engine/install/)
2. Run the following commands
    ```sh
    docker build -t jittermon . # build docker image
    docker compose up -d        # start the demo
    ```
3. Go to http://localhost:3000/d/aec2tnhcwbuo0b in your browser
4. Log in with username `demo` & password `demo`.

## TODOs
- Organize folders, nesting stuff like fly and docker out of root.
