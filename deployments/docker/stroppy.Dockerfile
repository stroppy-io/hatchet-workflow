FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o stroppy-edge ./cmd/edge/stroppy

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/stroppy-edge .

CMD ["./stroppy-edge"]
