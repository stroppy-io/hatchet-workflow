# Stage 1: Build the web SPA (served at runtime from STROPPY_WEB_DIR).
FROM node:22-alpine AS spa

WORKDIR /app/web
COPY web/package.json web/yarn.lock* ./
RUN yarn install --frozen-lockfile 2>/dev/null || yarn install
COPY web/ .
RUN npx vite build

# Stage 2: Build the Go binary + download Terraform.
FROM golang:1.25.5-alpine AS build

WORKDIR /app

ARG VERSION=dev
ARG TERRAFORM_VERSION=1.14.5

# Install terraform from the Yandex Cloud mirror (used by the YC provisioner).
RUN apk add --no-cache wget unzip && \
    wget https://hashicorp-releases.yandexcloud.net/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    mv terraform /usr/local/bin/terraform && \
    rm terraform_${TERRAFORM_VERSION}_linux_amd64.zip

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Version is injected at link time (build.Version → logged at startup).
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod \
	-ldflags="-w -s \
	  -X github.com/stroppy-io/stroppy-cloud/internal/build.Version=${VERSION} \
	  -X github.com/stroppy-io/stroppy-cloud/internal/build.ServiceName=stroppy-cloud" \
	-trimpath \
	-o /app/bin/stroppy-cloud "./cmd/stroppy-cloud"

# Stage 3: Runtime
FROM ubuntu:22.04

ARG STROPPY_VERSION=4.1.0

RUN apt-get update && apt-get install -y --no-install-recommends \
	bash curl ca-certificates wget sudo gnupg lsb-release \
	&& rm -rf /var/lib/apt/lists/*

# Install stroppy CLI (needed for the authoring probe API).
RUN curl -fsSL "https://github.com/stroppy-io/stroppy/releases/download/v${STROPPY_VERSION}/stroppy_linux_amd64.tar.gz" \
    | tar -xz -C /tmp \
    && cp /tmp/stroppy /usr/local/bin/stroppy \
    && chmod +x /usr/local/bin/stroppy \
    && rm -rf /tmp/*

COPY --from=build /usr/local/bin/terraform /usr/local/bin/terraform
COPY --from=build /app/bin/stroppy-cloud /usr/local/bin/stroppy-cloud
# The SPA is served from STROPPY_WEB_DIR (filesystem, not go:embed).
COPY --from=spa /app/web/dist /srv/web
ENV STROPPY_WEB_DIR=/srv/web

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/stroppy-cloud"]
