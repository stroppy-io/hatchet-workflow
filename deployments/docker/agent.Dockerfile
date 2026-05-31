# Stroppy Agent image — emulates a cloud VM 1:1.
#
# This is a REAL systemd machine (PID 1 = systemd), exactly like a provisioned
# cloud VM. The image bakes NO agent binary: on boot the stroppy-agent systemd
# service downloads the agent from the server (ExecStartPre, GET
# $STROPPY_SERVER_ADDR/agent/binary) and runs it. The agent then connects to
# Temporal through the server's gRPC proxy and may apt-install databases directly
# into the VM (apt routed through the server's cache). The deployer drops
# /etc/stroppy-agent.env (the agent's single config: the server address) + the
# apt proxy config before starting the container — like cloud-init userdata.
#
# Requires the container to run with systemd's needs (privileged, cgroup mount,
# tmpfs /run) — set by the deployer at run time.

FROM jrei/systemd-ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Tools the VM needs to bootstrap + later apt-install databases.
RUN apt-get update && apt-get install -y --no-install-recommends \
        bash ca-certificates curl gnupg lsb-release sudo apt-utils \
    && rm -rf /var/lib/apt/lists/*

# stroppy-agent systemd service. ExecStartPre downloads the agent binary from the
# server; EnvironmentFile carries the server address the deployer wrote. The agent
# is the ONLY thing the VM runs beyond the base system; Restart=always keeps it up.
RUN printf '[Unit]\n\
Description=Stroppy Agent\n\
After=network-online.target\n\
Wants=network-online.target\n\
\n\
[Service]\n\
Type=simple\n\
EnvironmentFile=-/etc/stroppy-agent.env\n\
ExecStartPre=/bin/bash -c "curl -fL --retry 30 --retry-delay 2 $${STROPPY_SERVER_ADDR}/agent/binary -o /usr/local/bin/stroppy-agent && chmod +x /usr/local/bin/stroppy-agent"\n\
ExecStart=/usr/local/bin/stroppy-agent\n\
Restart=always\n\
RestartSec=2\n\
\n\
[Install]\n\
WantedBy=multi-user.target\n' > /etc/systemd/system/stroppy-agent.service \
    && systemctl enable stroppy-agent

# PID 1 is systemd (from the base image). The deployer writes
# /etc/stroppy-agent.env (STROPPY_SERVER_ADDR=...) before start.
