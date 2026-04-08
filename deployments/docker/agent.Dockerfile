FROM jrei/systemd-ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Pre-install common tools.
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash curl ca-certificates wget sudo gnupg lsb-release python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install vmagent + node_exporter + stroppy at build time.
ARG VMAGENT_VERSION=v1.139.0
ARG NODE_EXPORTER_VERSION=1.9.1
ARG MYSQLD_EXPORTER_VERSION=0.19.0
ARG STROPPY_VERSION=4.1.0

RUN curl -fsSL "https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/${VMAGENT_VERSION}/vmutils-linux-amd64-${VMAGENT_VERSION}.tar.gz" \
    | tar xzf - -C /tmp \
    && cp /tmp/vmagent-prod /usr/local/bin/vmagent \
    && chmod +x /usr/local/bin/vmagent \
    && curl -fsSL "https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz" \
    | tar xzf - -C /tmp \
    && cp /tmp/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64/node_exporter /usr/local/bin/node_exporter \
    && chmod +x /usr/local/bin/node_exporter \
    && curl -fsSL "https://github.com/prometheus/mysqld_exporter/releases/download/v${MYSQLD_EXPORTER_VERSION}/mysqld_exporter-${MYSQLD_EXPORTER_VERSION}.linux-amd64.tar.gz" \
    | tar xzf - -C /tmp \
    && cp /tmp/mysqld_exporter-${MYSQLD_EXPORTER_VERSION}.linux-amd64/mysqld_exporter /usr/local/bin/mysqld_exporter \
    && chmod +x /usr/local/bin/mysqld_exporter \
    && curl -fsSL "https://github.com/stroppy-io/stroppy/releases/download/v${STROPPY_VERSION}/stroppy_linux_amd64.tar.gz" \
    | tar xzf - -C /tmp \
    && cp /tmp/stroppy /usr/local/bin/stroppy \
    && chmod +x /usr/local/bin/stroppy \
    && rm -rf /tmp/* \
    && chmod 1777 /tmp

# Create systemd unit for stroppy-agent.
# ExecStartPre downloads the agent binary from the server (URL passed via env).
# EnvironmentFile reads env vars set by the deployer at container creation.
RUN printf '[Unit]\n\
Description=Stroppy Agent\n\
After=network.target\n\
\n\
[Service]\n\
Type=simple\n\
EnvironmentFile=-/etc/stroppy-agent.env\n\
ExecStartPre=/bin/bash -c "curl -fL --retry 5 --retry-delay 2 $${STROPPY_SERVER_ADDR}/agent/binary -o /usr/local/bin/stroppy-agent && chmod +x /usr/local/bin/stroppy-agent"\n\
ExecStart=/usr/local/bin/stroppy-agent agent\n\
Restart=always\n\
\n\
[Install]\n\
WantedBy=multi-user.target\n' > /etc/systemd/system/stroppy-agent.service \
    && systemctl enable stroppy-agent

# systemd is PID 1 (from base image CMD ["/lib/systemd/systemd"])
