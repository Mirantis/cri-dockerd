# syntax=docker/dockerfile:1

# ---- Build Stage ----
ARG GO_VERSION=1.24.9
FROM golang:${GO_VERSION}-bookworm AS builder

ARG VERSION=""
ARG REVISION=""
ARG PRERELEASE=""
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /go/src/github.com/Mirantis/cri-dockerd

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags "-s -w \
    -X github.com/Mirantis/cri-dockerd/cmd/version.Version=${VERSION} \
    -X github.com/Mirantis/cri-dockerd/cmd/version.PreRelease=${PRERELEASE} \
    -X github.com/Mirantis/cri-dockerd/cmd/version.GitCommit=${REVISION}" \
    -o /usr/local/bin/cri-dockerd

# ---- Test Stage ----
FROM builder AS test
RUN go test ./...

# ---- Final Stage ----
FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /usr/local/bin/cri-dockerd /usr/local/bin/cri-dockerd

ENTRYPOINT ["cri-dockerd"]

