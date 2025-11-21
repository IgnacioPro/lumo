# Multi-stage build for Lumo Agent
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the agent
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty)" \
    -o lumo-agent \
    ./cmd/lumo-agent

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && update-ca-certificates

# Create non-root user
RUN addgroup -g 1000 lumo && \
    adduser -D -u 1000 -G lumo lumo

# Copy binary from builder
COPY --from=builder /build/lumo-agent /usr/local/bin/lumo-agent

# Set permissions
RUN chmod +x /usr/local/bin/lumo-agent

# Note: In DaemonSet mode, the pod runs as root to access host resources
# This is configured in the Kubernetes manifest, not here
USER lumo

# Expose health and metrics ports
EXPOSE 8080 9090

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/lumo-agent"]
