FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o master-worker ./cmd/master

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/master-worker .

CMD ["./master-worker"]
