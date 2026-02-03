FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o postgres-edge ./cmd/edge/postgres

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/postgres-edge .

CMD ["./postgres-edge"]
