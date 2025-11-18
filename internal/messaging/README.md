# Lumo Messaging System

Phase 11 implementation - Pub/Sub messaging integration with support for multiple providers.

## Overview

The messaging system enables Lumo agents to publish diagnostic results and other events to messaging platforms for distributed processing. It provides a unified interface for pub/sub operations across different messaging providers.

## Supported Providers

- **NATS**: Lightweight, high-performance messaging with built-in clustering
- **Kafka**: Distributed streaming platform for high-throughput scenarios
- **RabbitMQ**: Feature-rich message broker with flexible routing
- **Redis**: Simple pub/sub using Redis (already used for caching)

## Message Topics

The system uses topic-based routing with 5 predefined topics:

- `lumo.diagnostics` - Diagnostic results from agents
- `lumo.remediation` - Remediation actions and results
- `lumo.alerts` - System alerts and warnings
- `lumo.lifecycle` - Agent lifecycle events (startup, shutdown, heartbeat)
- `lumo.metrics` - Agent metrics and performance data

## Configuration

### Basic Configuration

```yaml
agent:
  messaging:
    enabled: true
    provider: nats  # or kafka, rabbitmq, redis
    url: nats://localhost:4222
    timeout: 30s
```

### With TLS

```yaml
agent:
  messaging:
    enabled: true
    provider: nats
    url: nats://localhost:4222
    tls:
      enabled: true
      cert_file: /path/to/cert.pem
      key_file: /path/to/key.pem
      ca_file: /path/to/ca.pem
      insecure_skip_verify: false
```

### With Authentication

```yaml
agent:
  messaging:
    enabled: true
    provider: kafka
    url: localhost:9092
    username: lumo
    password: secret  # Better: use LUMO_AGENT_MESSAGING_PASSWORD env var
```

### Advanced Configuration

```yaml
agent:
  messaging:
    enabled: true
    provider: rabbitmq
    url: amqp://localhost:5672
    timeout: 30s
    retry:
      max_attempts: 3
      base_delay: 1s
      max_delay: 30s
    dlq:
      enabled: true
      topic: lumo.dlq
      max_size: 10000
```

## Provider-Specific URLs

- **NATS**: `nats://localhost:4222` or `nats://user:pass@localhost:4222`
- **Kafka**: `localhost:9092` (single broker) or `broker1:9092,broker2:9092` (multiple)
- **RabbitMQ**: `amqp://localhost:5672` or `amqps://user:pass@localhost:5672`
- **Redis**: `redis://localhost:6379/0` or `redis://localhost:6379/1`

## Usage Example

```go
// Create messaging config
msgCfg := messaging.FromAgentConfig(&cfg.Agent.Messaging)

// Create publisher
publisher, err := messaging.NewPublisher(msgCfg, logger)
if err != nil {
    return err
}
defer publisher.Close()

// Publish a message
msg := messaging.NewMessage(messaging.TopicDiagnostics, map[string]interface{}{
    "report": report,
    "checks": []string{"cpu", "memory"},
})
msg.AgentID = agentID.String()
msg.Hostname = hostname

err = publisher.Publish(ctx, msg)
```

## NoOp Mode

When messaging is disabled (`enabled: false`), the system uses a NoOp publisher/subscriber that does nothing. This allows the agent to run without messaging infrastructure.

## Dependencies

The messaging system requires the following Go packages:

- `github.com/nats-io/nats.go` v1.37.0 - for NATS support
- `github.com/segmentio/kafka-go` v0.4.47 - for Kafka support
- `github.com/rabbitmq/amqp091-go` v1.10.0 - for RabbitMQ support
- `github.com/redis/go-redis/v9` v9.16.0 - for Redis support (already present)

## Testing

Run tests:

```bash
go test ./internal/messaging/...
```

Run with coverage:

```bash
go test -cover ./internal/messaging/...
```

## Architecture

```
internal/messaging/
├── messaging.go       # Core interfaces (Publisher, Subscriber, Message)
├── publisher.go       # Factory + NoOp implementations
├── config.go          # Config conversion utilities
├── adapter.go         # Provider config adapter
└── providers/
    ├── nats.go        # NATS implementation
    ├── kafka.go       # Kafka implementation
    ├── rabbitmq.go    # RabbitMQ implementation
    └── redis.go       # Redis implementation
```

## Features

- **Unified Interface**: Single API for all providers
- **Type Safety**: Strong typing with Go interfaces
- **Error Handling**: Graceful degradation with retry logic
- **TLS Support**: Secure communication for all providers
- **Authentication**: Username/password and token-based auth
- **Dead Letter Queue**: Failed message handling
- **Batch Publishing**: Efficient batch operations (where supported)
- **Auto-Reconnect**: Automatic reconnection on network failures
- **Context Support**: Proper context handling for cancellation

## Performance

- **NATS**: < 1ms latency, 10M+ msgs/sec
- **Kafka**: High throughput, batch-oriented
- **RabbitMQ**: Feature-rich, moderate throughput
- **Redis**: Simple, fast for low-volume use cases

## Integration with Agent

The agent automatically publishes diagnostic results to messaging when enabled:

1. Agent runs diagnostics
2. Results are published to `lumo.diagnostics` topic
3. Fallback to API submission if messaging fails
4. Cache results locally if both fail (offline mode)

## Troubleshooting

### Provider Not Available

If you see "provider not available" errors, ensure:
1. Messaging is enabled in config
2. Provider URL is correct and reachable
3. Authentication credentials are valid
4. TLS certificates are properly configured

### Connection Failures

Check:
1. Network connectivity to messaging server
2. Firewall rules allow the connection
3. Server is running and accepting connections
4. Credentials and permissions are correct

### Performance Issues

Optimize:
1. Use batch publishing where possible
2. Adjust retry settings
3. Consider message compression
4. Monitor server resources
5. Use local caching (offline mode)

## Future Enhancements

- Message compression (gzip, snappy)
- Schema validation (JSON Schema, Protobuf)
- Message prioritization
- Consumer group management
- Message filtering and routing
- Metrics and monitoring integration
- Additional providers (MQTT, AMQP 1.0, Azure Service Bus)
