FROM golang:1.25.5-alpine AS base

WORKDIR /app

ARG VERSION=0.0.0
ARG TERRAFORM_VERSION=1.14.5

RUN apk add --no-cache wget unzip && \
    wget https://hashicorp-releases.yandexcloud.net/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    mv terraform /usr/local/bin/terraform && \
    rm terraform_${TERRAFORM_VERSION}_linux_amd64.zip

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPRIVATE="github.com/stroppy-io" go build \
	-ldflags="-w -s -X github.com/stroppy-io/hatchet-workflow/internal/core/build.Version=$VERSION -X github.com/stroppy-io/hatchet-workflow/internal/core/build.ServiceName=master-worker" \
	-trimpath \
	-v -o /app/bin/master-worker "./cmd/master-worker"

FROM gcr.io/distroless/static-debian11

WORKDIR /root/

COPY --from=base /usr/local/bin/terraform /usr/local/bin/terraform
COPY --from=base /app/bin/master-worker .

CMD ["./master-worker"]
