# Lean demo server image (no SPA) for the docker-provider e2e.
#
# Copies HOST-built linux binaries (built by `make demo-up` where the go module
# cache + git are available) — avoids fetching the private schemapb module inside
# the build. Ships the control-plane CLI (gateway + services), the linux agent
# binary the gateway serves at GET /agent/binary, and a stand-in stroppy served
# as the "stroppy" artifact so the whole pipeline runs without minio.
#
# Build prerequisites (make demo-up):
#   GOOS=linux GOARCH=amd64 go build -o bin/stroppy-cloud-linux ./cmd/cli
#   GOOS=linux GOARCH=amd64 go build -o bin/stroppy-agent-linux ./cmd/agent

FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

COPY bin/stroppy-cloud-linux /usr/local/bin/stroppy-cloud
# The agent binary the gateway hands to bootstrapping containers.
COPY bin/stroppy-agent-linux /usr/local/bin/stroppy-agent-payload
COPY deployments/docker/stand-in-stroppy.sh /usr/local/share/stroppy-stand-in
RUN chmod +x /usr/local/bin/stroppy-cloud /usr/local/bin/stroppy-agent-payload /usr/local/share/stroppy-stand-in

ENV AGENT_BINARY_PATH=/usr/local/bin/stroppy-agent-payload
ENV STROPPY_UPSTREAM=/usr/local/share/stroppy-stand-in

EXPOSE 8080 8081 3142
ENTRYPOINT ["/usr/local/bin/stroppy-cloud"]
