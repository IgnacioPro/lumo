# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X github.com/ignacio/lumo/internal/version.Version=${VERSION}" -o bin/lumo ./cmd/lumo

# Final stage
FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/bin/lumo /app/lumo

# Expose API port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/app/lumo"]
CMD ["server"]
