# Build stage
FROM golang:1.24-alpine AS builder
ARG VERSION=dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X github.com/ignacio/lumo/internal/version.Version=${VERSION}" -o lumo ./cmd/lumo

# Final stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/lumo .
RUN apk --no-cache add ca-certificates
EXPOSE 8080
ENTRYPOINT ["./lumo"]
