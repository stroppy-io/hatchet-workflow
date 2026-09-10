# syntax=docker/dockerfile:1.7
# The server image: the stroppy-cloud binary with the embedded SPA plus the
# pipeline binaries it pushes into every tenant namespace (STROPPY.MD §7).
# One image = one revision of both.

ARG GO_VERSION=1.26
ARG NODE_VERSION=22

# --- web --------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS web
WORKDIR /src/web
COPY web/package.json web/yarn.lock ./
RUN yarn install --frozen-lockfile
COPY web/ ./
RUN yarn build

# --- go ---------------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
COPY pipelines/go.mod pipelines/go.sum ./pipelines/
RUN go mod download && cd pipelines && go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath \
      -ldflags="-w -s -X github.com/stroppy-io/stroppy-cloud/internal/build.Version=${VERSION} -X github.com/stroppy-io/stroppy-cloud/internal/build.Commit=${COMMIT} -X github.com/stroppy-io/stroppy-cloud/internal/build.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      -o /out/stroppy-cloud ./cmd/stroppy-cloud/
RUN cd pipelines && for p in run suite provider-verify quotas; do \
      go build -trimpath -ldflags="-w -s" -o /out/pipelines/stroppy-$p ./cmd/$p/; \
    done

# --- runtime ----------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/stroppy-cloud /usr/local/bin/stroppy-cloud
COPY --from=build /out/pipelines /opt/stroppy/pipelines
ENV STROPPY_HTTP_ADDR=:8080 \
    STROPPY_INFRA_PIPELINES_DIR=/opt/stroppy/pipelines
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/stroppy-cloud"]
