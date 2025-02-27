ARG GO_VERSION=1.23
ARG ALPINE_VERSION=3.19

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder
ARG DOCKER_META_VERSION=dev
RUN --mount=type=bind,target=/app,source=. \
    --mount=type=cache,target=/go/pkg/mod \
    <<EOT
    set -ex
    cd /app
    export CGO_ENABLED=0
    export GOOS=linux
    for GOARCH in amd64 arm64; do
        export GOARCH
        go build -o /sentrytunnel-$GOOS-$GOARCH -ldflags="-s -X main.Version=${DOCKER_META_VERSION}" cmd/sentrytunnel/sentrytunnel.go
    done
EOT

FROM quay.io/prometheus/busybox-${TARGETOS}-${TARGETARCH}:latest
ARG TARGETOS
ARG TARGETARCH
COPY --from=builder /sentrytunnel-$TARGETOS-$TARGETARCH /usr/local/bin/sentrytunnel
ENTRYPOINT [ "/usr/local/bin/sentrytunnel" ]
