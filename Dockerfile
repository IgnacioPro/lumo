# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for build info
RUN apk add --no-cache git

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binaries
# -trimpath: remove file system paths from executable
# -ldflags: strip debug info (-s -w) and set version
ARG VERSION=dev
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o lumo ./cmd/lumo

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o lumo-agent ./cmd/lumo-agent

# Stage 2: Final (Distroless)
FROM gcr.io/distroless/static:nonroot

WORKDIR /

# Copy binaries
COPY --from=builder /app/lumo /usr/bin/lumo
COPY --from=builder /app/lumo-agent /usr/bin/lumo-agent

# Run as non-root user
USER nonroot:nonroot

# Default to agent, but can be overridden
ENTRYPOINT ["/usr/bin/lumo-agent"]
