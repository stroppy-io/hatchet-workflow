FROM golang:1.25.5-alpine AS base

WORKDIR /app

ARG VERSION=0.0.0

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPRIVATE="github.com/stroppy-io" go build \
	-ldflags="-w -s -X github.com/stroppy-io/hatchet-workflow/internal/core/build.Version=$VERSION -X github.com/stroppy-io/hatchet-workflow/internal/core/build.ServiceName=master-worker" \
	-trimpath \
	-v -o /app/bin/master-worker "./cmd/master-worker"

FROM gcr.io/distroless/static-debian11

WORKDIR /root/

COPY --from=base /app/bin/master-worker .

CMD ["./master-worker"]
