FROM golang:1.25.5-alpine AS base

WORKDIR /app

ARG VERSION=0.0.0

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPRIVATE="github.com/stroppy-io" go build \
	-ldflags="-w -s -X github.com/stroppy-io/hatchet-workflow/internal/core/build.Version=$VERSION -X github.com/stroppy-io/hatchet-workflow/internal/core/build.ServiceName=edge-worker" \
	-trimpath \
	-v -o /app/bin/edge-worker "./cmd/edge-worker"

FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
	bash curl ca-certificates wget sudo gnupg lsb-release \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=base /app/bin/edge-worker /usr/local/bin/edge-worker

ENTRYPOINT ["/usr/local/bin/edge-worker"]
