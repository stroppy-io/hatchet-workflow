# Stage 1: Build frontend
FROM node:20-alpine AS frontend

WORKDIR /web
COPY web/package.json web/yarn.lock ./
RUN yarn install --frozen-lockfile --ignore-engines
COPY web/ .
RUN yarn build

# Stage 2: Build Go binary with embedded frontend
FROM golang:1.25.5-alpine AS backend

WORKDIR /app

ARG VERSION=0.0.0

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy built frontend into the embed directory
COPY --from=frontend /web/dist/ ./internal/domain/api/static/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPRIVATE="github.com/stroppy-io" go build \
	-ldflags="-w -s -X github.com/stroppy-io/hatchet-workflow/internal/core/build.Version=$VERSION -X github.com/stroppy-io/hatchet-workflow/internal/core/build.ServiceName=api-server" \
	-trimpath \
	-v -o /app/bin/api-server "./cmd/api-server"

# Stage 3: Minimal runtime
FROM gcr.io/distroless/static-debian11

WORKDIR /root/

COPY --from=backend /app/bin/api-server .

EXPOSE 8888

CMD ["./api-server"]
