# Build stage
FROM golang:1.25-alpine3.21 AS builder

ARG VERSION=dev

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build with optimizations
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X github.com/ignacio/lumo/internal/version.Version=${VERSION}" \
    -trimpath \
    -o lumo ./cmd/lumo

# Compress binary with UPX (reduces size ~50-70%)
RUN apk add --no-cache upx && upx --best --lzma lumo

# Final stage - scratch for minimal size (~5-10MB total)
FROM scratch

# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /app/lumo /lumo

EXPOSE 8080

ENTRYPOINT ["/lumo"]
