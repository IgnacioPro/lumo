# Messaging System

## Overview

The Lumo messaging system provides a unified pub/sub framework for high-throughput event distribution between agents and the API server. It replaces direct HTTP POST for event submission, enabling better scalability and reliability.

**Key Features:**
- **Multi-provider support**: NATS, Kafka, RabbitMQ, Redis Streams
- **Unified interface**: Switch providers without code changes
- **Reliability**: Dead-letter queues, retries, acknowledgments
- **Observability**: OpenTelemetry tracing, Prometheus metrics
- **Circuit breakers**: Automatic fault isolation

## Architecture

```
┌─────────────┐      Publish      ┌──────────────┐      Subscribe      ┌─────────────┐
│   Agent     │ ────────────────► │   Message    │ ──────────────────► │ API Server  │
│ (Publisher) │                    │    Broker    │                     │ (Subscriber)│
└─────────────┘                    └──────────────┘                     └─────────────┘
                                           │
                                           │ Failed Messages
                                           ▼
                                   ┌──────────────┐
                                   │ Dead-Letter  │
                                   │    Queue     │
                                   └──────────────┘
```

## Provider Comparison

| Provider  | Best For                  | Throughput  | Latency | Key Features                    |
|-----------|---------------------------|-------------|---------|---------------------------------|
| NATS      | Low latency, simplicity   | High        | <1ms    | JetStream, KV store, clustering |
| Kafka     | High throughput, replay   | Very High   | ~5ms    | Partitions, replay, durability  |
| RabbitMQ  | Complex routing, enterprise| Medium     | ~3ms    | DLX, routing keys, plugins      |
| Redis     | Existing Redis infra      | Medium      | <2ms    | Simple setup, in-memory         |

**Recommendation:**
- **NATS**: Default choice for most deployments (simple, fast, reliable)
- **Kafka**: High-volume environments (>10k events/sec) with replay requirements
- **RabbitMQ**: Existing RabbitMQ infrastructure or complex routing needs
- **Redis**: Already using Redis for caching, want minimal additional infrastructure

## Configuration

### Basic Setup (NATS)

```yaml
messaging:
  enabled: true
  provider: nats
  brokers:
    - "nats://localhost:4222"
  event_topic: "lumo.events"
  dead_letter_topic: "lumo.events.dlq"
  max_retries: 3
  retry_backoff: "5s"
  consumer_group: "lumo-api-consumer"
```

### Kafka Configuration

```yaml
messaging:
  enabled: true
  provider: kafka
  brokers:
    - "localhost:9092"
    - "localhost:9093"  # Multiple brokers for HA
  event_topic: "lumo.events"
  dead_letter_topic: "lumo.events.dlq"
  max_retries: 3
  consumer_group: "lumo-api-consumer"
  max_in_flight: 100  # Higher throughput
```

### RabbitMQ Configuration

```yaml
messaging:
  enabled: true
  provider: rabbitmq
  brokers:
    - "localhost:5672"
  username: "lumo"
  password_env_var: "LUMO_MESSAGING_PASSWORD"
  tls: true
  event_topic: "lumo.events"
  dead_letter_topic: "lumo.events.dlq"
```

### Redis Streams Configuration

```yaml
messaging:
  enabled: true
  provider: redis
  brokers:
    - "localhost:6379"
  password_env_var: "LUMO_MESSAGING_PASSWORD"
  event_topic: "lumo.events"
  dead_letter_topic: "lumo.events.dlq"
  consumer_group: "lumo-api-consumer"
```

## Usage Examples

### Publisher (Agent)

```go
package main

import (
    "context"
    "github.com/ignacio/lumo/internal/messaging"
)

func publishEvent() error {
    // Create messaging config from application config
    msgConfig := &messaging.Config{
        Provider:        "nats",
        Brokers:         []string{"nats://localhost:4222"},
        EventTopic:      "lumo.events",
        DeadLetterTopic: "lumo.events.dlq",
        MaxRetries:      3,
    }

    // Create provider and publisher
    provider, err := messaging.NewProvider(msgConfig)
    if err != nil {
        return err
    }

    publisher, err := provider.NewPublisher(msgConfig)
    if err != nil {
        return err
    }
    defer publisher.Close()

    // Publish event
    eventData := []byte(`{"type":"cpu-high","severity":"critical"}`)
    ctx := context.Background()
    
    return publisher.Publish(ctx, msgConfig.EventTopic, eventData)
}

// Batch publishing for better throughput
func publishBatch() error {
    // ... setup as above
    
    messages := [][]byte{
        []byte(`{"type":"cpu-high","severity":"critical"}`),
        []byte(`{"type":"memory-low","severity":"warning"}`),
        []byte(`{"type":"disk-full","severity":"critical"}`),
    }
    
    return publisher.PublishBatch(ctx, msgConfig.EventTopic, messages)
}
```

### Subscriber (API Server)

```go
package main

import (
    "context"
    "github.com/ignacio/lumo/internal/messaging"
)

func subscribeToEvents() error {
    // Create messaging config
    msgConfig := &messaging.Config{
        Provider:       "nats",
        Brokers:        []string{"nats://localhost:4222"},
        EventTopic:     "lumo.events",
        ConsumerGroup:  "lumo-api-consumer",
    }

    // Create provider and subscriber
    provider, err := messaging.NewProvider(msgConfig)
    if err != nil {
        return err
    }

    subscriber, err := provider.NewSubscriber(msgConfig)
    if err != nil {
        return err
    }
    defer subscriber.Close()

    // Define message handler
    handler := func(ctx context.Context, msg *messaging.Message) error {
        log.Printf("Received event: %s", string(msg.Data))
        // Process event (AI analysis, notifications, storage)
        return processEvent(msg.Data)
    }

    // Subscribe to topic
    ctx := context.Background()
    return subscriber.Subscribe(ctx, msgConfig.EventTopic, handler)
}
```

## Deployment

### NATS Deployment (Kubernetes)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nats
  namespace: lumo-system
spec:
  selector:
    app: nats
  ports:
    - port: 4222
      name: client
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: nats
  namespace: lumo-system
spec:
  serviceName: nats
  replicas: 3
  selector:
    matchLabels:
      app: nats
  template:
    metadata:
      labels:
        app: nats
    spec:
      containers:
      - name: nats
        image: nats:2.10-alpine
        args:
          - "-js"  # Enable JetStream
          - "-sd=/data"  # Data directory
        ports:
        - containerPort: 4222
          name: client
        volumeMounts:
        - name: data
          mountPath: /data
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

### Agent Configuration (Use Messaging)

```yaml
# deployments/kubernetes/base/configmap-agent.yaml
messaging:
  enabled: true
  provider: nats
  brokers:
    - "nats.lumo-system.svc.cluster.local:4222"
  event_topic: "lumo.events"
  consumer_group: "lumo-agent"
```

### API Server Configuration

```yaml
# API server consumes events from messaging system
messaging:
  enabled: true
  provider: nats
  brokers:
    - "nats.lumo-system.svc.cluster.local:4222"
  event_topic: "lumo.events"
  dead_letter_topic: "lumo.events.dlq"
  consumer_group: "lumo-api-consumer"
```

## Metrics

### Prometheus Metrics (per provider)

**Publisher Metrics:**
- `lumo_<provider>_publish_total{topic, status}` - Total messages published
- `lumo_<provider>_publish_duration_seconds{topic}` - Publish latency histogram

**Subscriber Metrics:**
- `lumo_<provider>_subscribe_total{topic, status}` - Total messages received

### Example Queries

```promql
# Publish success rate
rate(lumo_nats_publish_total{status="success"}[5m]) 
/ 
rate(lumo_nats_publish_total[5m])

# P99 publish latency
histogram_quantile(0.99, rate(lumo_nats_publish_duration_seconds_bucket[5m]))

# Failed message rate
rate(lumo_nats_subscribe_total{status="error"}[5m])
```

## Troubleshooting

### Connection Issues

**Symptom:** `Failed to connect to NATS`

**Solution:**
```bash
# Check broker is running
kubectl get pods -n lumo-system | grep nats

# Check service DNS
nslookup nats.lumo-system.svc.cluster.local

# Check connectivity
kubectl exec -it lumo-agent-xxx -- nc -zv nats.lumo-system.svc.cluster.local 4222
```

### Messages Not Being Received

**Symptom:** Publisher succeeds but subscriber doesn't receive messages

**NATS Troubleshooting:**
```bash
# Check stream exists
nats stream ls

# Check consumer exists
nats consumer ls LUMO_EVENTS

# Check pending messages
nats stream info LUMO_EVENTS
```

**Kafka Troubleshooting:**
```bash
# Check topic exists
kafka-topics.sh --list --bootstrap-server localhost:9092

# Check consumer group
kafka-consumer-groups.sh --describe --group lumo-api-consumer --bootstrap-server localhost:9092
```

### High Latency

**Symptoms:** Publish latency >100ms

**Diagnosis:**
1. Check network latency between agent and broker
2. Check broker resource usage (CPU, memory)
3. Review batch size and timeout settings
4. Check for circuit breaker activation

**Solutions:**
- Increase `max_in_flight` for higher throughput
- Use batch publishing for multiple events
- Add more broker instances for load distribution
- Tune provider-specific settings (batch size, compression)

### Dead-Letter Queue Growing

**Symptom:** Messages accumulating in DLQ

**Investigation:**
```bash
# Check DLQ messages (NATS)
nats stream info LUMO_EVENTS --subject lumo.events.dlq

# Peek at failed message
nats stream get LUMO_EVENTS --id 12345
```

**Common Causes:**
- Invalid message format (JSON parsing errors)
- Downstream service failures (database, AI provider)
- Handler crashes or panics
- Timeout exceeded

## Performance Tuning

### High Throughput (>1000 events/sec)

```yaml
messaging:
  provider: kafka  # Best for high throughput
  max_in_flight: 100  # Increase concurrent processing
  
  # Agent-side batching
  # (implement in api_processor.go)
```

### Low Latency (<10ms p99)

```yaml
messaging:
  provider: nats  # Lowest latency
  max_in_flight: 10
  retry_backoff: "1s"  # Faster retries
```

### High Availability

```yaml
messaging:
  brokers:  # Multiple brokers for redundancy
    - "nats-0.nats:4222"
    - "nats-1.nats:4222"
    - "nats-2.nats:4222"
  consumer_group: "lumo-api-consumer"  # Consumer group for HA
```

## Migration from HTTP

### Before (HTTP POST)

```go
// internal/agent/eventdriven/api_processor.go
func (p *APIProcessor) submitEvent(event *types.Event) error {
    resp, err := http.Post(p.apiURL, "application/json", bytes.NewReader(data))
    // ...
}
```

### After (Messaging)

```go
func (p *APIProcessor) submitEvent(event *types.Event) error {
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    
    return p.publisher.Publish(ctx, p.config.EventTopic, data)
}
```

## Security

### TLS Configuration

```yaml
messaging:
  tls: true
  # Provider-specific TLS settings
  # Certificates configured via env vars or K8s secrets
```

### Authentication

```yaml
messaging:
  username: "lumo-agent"
  password_env_var: "LUMO_MESSAGING_PASSWORD"  # Never hardcode passwords
```

**Kubernetes Secret:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: messaging-credentials
  namespace: lumo-system
stringData:
  password: "secure-password"
```

## Testing

See `tests/integration/messaging_test.go` for end-to-end tests and `tests/load/messaging_load_test.go` for performance benchmarks.

## References

- [NATS Documentation](https://docs.nats.io/)
- [Kafka Documentation](https://kafka.apache.org/documentation/)
- [RabbitMQ Documentation](https://www.rabbitmq.com/documentation.html)
- [Redis Streams Documentation](https://redis.io/docs/data-types/streams/)

---

## Deployment Profiles

**Don't know which provider to choose?** Use deployment profiles for automatic configuration based on scale.

### Quick Selection

```go
import "github.com/ignacio/lumo/internal/messaging"

// Automatic recommendation
profile := messaging.ProfileRecommendation(agentCount)
config, _ := messaging.ProfileConfig(profile)

provider, _ := messaging.NewProvider(config)
```

### Available Profiles

| Profile | Agents | Provider | Throughput | Cost/month | Setup Time |
|---------|--------|----------|------------|------------|------------|
| **Startup** | 0-10 | Redis | ~100/sec | $0 | 0 min |
| **Small Business** | 10-50 | NATS | 1,000+/sec | $7 | 5 min |
| **Enterprise** | 50-500 | NATS Cluster | 10,000+/sec | $60 | 15 min |
| **Hyperscale** | 500+ | Kafka | 100,000+/sec | $500 | 30 min |

**Startup Profile** - Uses Redis Streams (zero additional infrastructure)
```go
config, _ := messaging.ProfileConfig(messaging.ProfileStartup)
// Provider: redis, Brokers: [localhost:6379]
```

**Small Business Profile** - NATS single node (recommended default)
```go
config, _ := messaging.ProfileConfig(messaging.ProfileSmallBusiness)
// Provider: nats, Brokers: [nats://localhost:4222]
```

**Enterprise Profile** - NATS 3-node cluster (high availability)
```go
config, _ := messaging.ProfileConfig(messaging.ProfileEnterprise)
// Provider: nats, Brokers: [nats-0:4222, nats-1:4222, nats-2:4222]
```

**Hyperscale Profile** - Kafka cluster (extreme throughput)
```go
config, _ := messaging.ProfileConfig(messaging.ProfileHyperscale)
// Provider: kafka, Brokers: [kafka-0:9092, kafka-1:9092, kafka-2:9092]
```

### Why We Keep All Providers

**Different scales need different tools:**
- **Redis:** Already deployed? Use it (startup)
- **NATS:** Best for 90% of use cases (small-business, enterprise)
- **RabbitMQ:** Enterprise already using it? Easy migration
- **Kafka:** Global scale, multi-cluster, regulatory compliance

**You only deploy what you need** - profiles make this automatic.

See `/deployments/kubernetes/profiles/README.md` for deployment instructions.
