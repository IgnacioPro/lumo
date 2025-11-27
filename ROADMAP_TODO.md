# Lumo Technical Roadmap - Implementation Tasks

> **Version:** 1.1.0 Development Roadmap
> **Last Updated:** 2025-11-27
> **Target Release:** January 2026
> **Status:** Ready for Implementation

This document provides granular, actionable technical tasks for achieving the v1.1.0 roadmap. Each phase is broken down into specific implementation steps with file paths, function signatures, and dependencies.

**Quick Navigation:**
- [Phase 11c: Messaging Integration](#phase-11c-messaging-integration) ⚡ **PRIORITY #1**
- [Phase 14: Advanced Reporting](#phase-14-advanced-reporting) 📊 **PRIORITY #2**
- [Quick Wins](#quick-wins-high-impact-low-effort) 🎯 **START IMMEDIATELY**
- [Phase 17a: Multi-Cluster Orchestration](#phase-17a-multi-cluster-orchestration) 🌐
- [Test Coverage Enhancement](#test-coverage-enhancement)
- [Performance Optimization](#performance-optimization)

---

## Quick Wins (High Impact, Low Effort)

**Timeline:** 1-2 weeks | **Priority:** Start these immediately while planning Phase 11c

### QW-1: Documentation Gaps

**Estimated Time:** 3-4 days

#### QW-1.1: Helm Deployment Guide
- [ ] **File:** Create `/docs/helm-deployment.md`
- [ ] **Content:**
  - [ ] Complete helm values documentation (currently missing)
  - [ ] Production deployment checklist
  - [ ] Ingress configuration examples (nginx, traefik)
  - [ ] TLS/cert-manager setup guide
  - [ ] External secrets integration (Vault, AWS Secrets Manager, Azure Key Vault)
  - [ ] Multi-environment setup (dev, staging, production)
  - [ ] Troubleshooting common helm issues
- [ ] **Code Examples:**
  ```yaml
  # Example production values.yaml structure
  # Example ingress configuration
  # Example external secrets setup
  ```
- [ ] **Testing:** Deploy using guide on fresh cluster

#### QW-1.2: API Authentication Guide
- [ ] **File:** Create `/docs/api-auth-guide.md`
- [ ] **Content:**
  - [ ] JWT token generation examples (curl, Go, Python, Node.js)
  - [ ] Token refresh workflow with sequence diagrams
  - [ ] API key creation and scopes
  - [ ] Rate limiting behavior and headers
  - [ ] Common auth errors and solutions
  - [ ] Integration examples with popular tools (Postman, Insomnia, HTTPie)
- [ ] **Code Examples:**
  ```bash
  # curl examples for all endpoints
  # Token refresh example
  ```
  ```go
  // Go client authentication example
  ```
  ```python
  # Python client authentication example
  ```

#### QW-1.3: Troubleshooting Playbook
- [ ] **File:** Create `/docs/troubleshooting.md`
- [ ] **Content:**
  - [ ] Common K8s deployment issues
    - [ ] Agent pod CrashLoopBackOff → Check Redis connectivity
    - [ ] ImagePullBackOff → Verify registry credentials
    - [ ] API pod not receiving events → Check RBAC permissions
  - [ ] Database connection failures
  - [ ] Redis cache issues
  - [ ] Event-driven system debugging
    - [ ] Informer sync failures
    - [ ] Debouncer stuck events
    - [ ] Missing events investigation
  - [ ] Performance degradation scenarios
  - [ ] Log analysis examples with jq/grep patterns
- [ ] **Runbooks:** Step-by-step resolution guides for each issue

#### QW-1.4: Update Existing Examples
- [ ] **File:** Enhance `/examples/README.md`
- [ ] **Tasks:**
  - [ ] Add RAG system usage example
  - [ ] Add event-driven monitoring example
  - [ ] Add circuit breaker behavior example
  - [ ] Add messaging integration example (post-Phase 11c)
  - [ ] Cross-reference with Phase features

---

### QW-2: Configuration Validation Tool

**Estimated Time:** 1-2 days

#### QW-2.1: Enhance lumo doctor
- [ ] **File:** `internal/doctor/checks.go`
- [ ] **Add Checks:**
  - [ ] `CheckMessagingConfig()` - Validate messaging provider config
    ```go
    func CheckMessagingConfig(cfg *config.Config) error {
        // Validate provider selection
        // Check connection to NATS/Kafka/RabbitMQ/Redis
        // Verify topic/queue permissions
        return nil
    }
    ```
  - [ ] `CheckReportingConfig()` - Validate reporting database schema
    ```go
    func CheckReportingConfig(cfg *config.Config) error {
        // Verify reports table exists
        // Check metrics_history table
        // Validate template paths
        return nil
    }
    ```
  - [ ] `CheckMultiClusterConfig()` - Validate federation config (Phase 17a)

#### QW-2.2: Add Strict Validation Mode
- [ ] **File:** `cmd/lumo/doctor.go`
- [ ] **Add Flag:** `--strict` for production validation
  ```go
  doctorCmd.Flags().Bool("strict", false, "Enable strict validation for production")
  ```
- [ ] **Checks:**
  - [ ] Ensure JWT secret is strong (>32 chars, high entropy)
  - [ ] Verify TLS certificates are valid and not self-signed
  - [ ] Check API rate limits are configured
  - [ ] Validate all external service health (DB, Redis, messaging)

---

### QW-3: Test Coverage for High-Value Packages

**Estimated Time:** 3-4 days

#### QW-3.1: Database Repository Tests
- [ ] **File:** Create `/internal/database/repository/agent_test.go` (currently 0% coverage)
- [ ] **Tests:**
  - [ ] `TestCreateAgent()` - Success, duplicate handling
  - [ ] `TestGetAgent()` - Found, not found, invalid ID
  - [ ] `TestUpdateAgent()` - Full update, partial update
  - [ ] `TestDeleteAgent()` - Soft delete, cascade
  - [ ] `TestListAgents()` - Pagination, filtering, sorting
  - [ ] `TestUpdateHeartbeat()` - Timestamp update, stale agent detection
- [ ] **Estimated LOC:** 200-250 lines
- [ ] **Pattern:** Table-driven tests with testcontainers

#### QW-3.2: Database Repository Tests (Events)
- [ ] **File:** Create `/internal/database/repository/event_test.go`
- [ ] **Tests:**
  - [ ] `TestCreateEvent()` - Various event types, metadata
  - [ ] `TestGetEvent()` - By ID, by type, by severity
  - [ ] `TestListEvents()` - Time-range filtering, namespace filtering
  - [ ] `TestEventAggregation()` - Count by type, severity distribution
- [ ] **Estimated LOC:** 200-250 lines

#### QW-3.3: Database Repository Tests (Jobs)
- [ ] **File:** Create `/internal/database/repository/job_test.go`
- [ ] **Tests:**
  - [ ] `TestCreateJob()` - Diagnostic, remediation jobs
  - [ ] `TestUpdateJobStatus()` - State transitions, invalid transitions
  - [ ] `TestListJobs()` - Filtering by status, agent, timestamp
- [ ] **Estimated LOC:** 150-200 lines

---

### QW-4: Prometheus Metrics Completeness

**Estimated Time:** 1-2 days

#### QW-4.1: Add Missing Metrics
- [ ] **File:** `internal/agent/metrics.go`
- [ ] **New Metrics:**
  ```go
  // Message queue metrics (add post-Phase 11c)
  messagePublishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
      Name: "lumo_message_publish_total",
      Help: "Total messages published to queue",
  }, []string{"topic", "status"})

  messagePublishDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
      Name: "lumo_message_publish_duration_seconds",
      Help: "Message publish latency",
      Buckets: prometheus.DefBuckets,
  }, []string{"topic"})

  // Diagnostic runner latency
  diagnosticRunnerDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
      Name: "lumo_diagnostic_runner_duration_seconds",
      Help: "Diagnostic runner execution time by checker",
      Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
  }, []string{"checker"})

  // Event processing rate
  eventProcessingRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
      Name: "lumo_event_processing_rate_per_min",
      Help: "Current event processing rate",
  }, []string{"event_type"})
  ```

#### QW-4.2: Add Metrics to Diagnostic Runner
- [ ] **File:** `internal/diagnostics/diagnostics.go`
- [ ] **Instrument:**
  ```go
  func (r *Runner) RunCheck(checker Checker) error {
      start := time.Now()
      defer func() {
          duration := time.Since(start).Seconds()
          diagnosticRunnerDuration.WithLabelValues(checker.Name()).Observe(duration)
      }()
      // ... existing code
  }
  ```

---

## Phase 11c: Messaging Integration

**Priority:** #1 ⚡ **CRITICAL PATH**
**Timeline:** 2-3 weeks
**Estimated LOC:** ~1,500 lines + 800 test lines
**Dependencies:** None - foundation ready

**Success Metrics:**
- [ ] 1,000 events/sec sustained throughput
- [ ] <10ms publish latency (p99)
- [ ] Zero message loss with dead-letter queue
- [ ] All 4 providers tested and working

---

### Week 1: Framework Design + NATS Implementation

#### MSG-1.1: Core Abstractions
**Days 1-2**

- [ ] **File:** Create `/internal/messaging/interface.go`
  ```go
  package messaging

  import (
      "context"
      "time"
  )

  // Publisher sends messages to topics
  type Publisher interface {
      Publish(ctx context.Context, topic string, message []byte) error
      PublishBatch(ctx context.Context, topic string, messages [][]byte) error
      Close() error
  }

  // Subscriber receives messages from topics
  type Subscriber interface {
      Subscribe(ctx context.Context, topic string, handler MessageHandler) error
      SubscribeMulti(ctx context.Context, topics []string, handler MessageHandler) error
      Unsubscribe(topic string) error
      Close() error
  }

  // MessageHandler processes received messages
  type MessageHandler func(ctx context.Context, msg *Message) error

  // Message represents a message from the queue
  type Message struct {
      ID        string
      Topic     string
      Data      []byte
      Headers   map[string]string
      Timestamp time.Time
      Attempt   int // For retry tracking
  }

  // Config holds messaging configuration
  type Config struct {
      Provider       string // nats, kafka, rabbitmq, redis
      Brokers        []string
      Username       string
      Password       string
      TLS            bool
      MaxRetries     int
      RetryBackoff   time.Duration
      DeadLetterTopic string
  }

  // Provider creates Publisher and Subscriber instances
  type Provider interface {
      NewPublisher(config *Config) (Publisher, error)
      NewSubscriber(config *Config) (Subscriber, error)
      Health() error
  }
  ```

- [ ] **File:** Create `/internal/messaging/factory.go`
  ```go
  package messaging

  import "fmt"

  // NewProvider creates a messaging provider
  func NewProvider(config *Config) (Provider, error) {
      switch config.Provider {
      case "nats":
          return &NATSProvider{}, nil
      case "kafka":
          return &KafkaProvider{}, nil
      case "rabbitmq":
          return &RabbitMQProvider{}, nil
      case "redis":
          return &RedisProvider{}, nil
      default:
          return nil, fmt.Errorf("unsupported messaging provider: %s", config.Provider)
      }
  }
  ```

#### MSG-1.2: NATS Provider Implementation
**Days 2-3**

- [ ] **File:** Create `/internal/messaging/providers/nats.go`
  ```go
  package providers

  import (
      "context"
      "github.com/nats-io/nats.go"
      "github.com/ignacio/lumo/internal/messaging"
  )

  type NATSProvider struct{}

  type NATSPublisher struct {
      conn *nats.Conn
      js   nats.JetStreamContext
      config *messaging.Config
  }

  type NATSSubscriber struct {
      conn *nats.Conn
      js   nats.JetStreamContext
      subs map[string]*nats.Subscription
  }

  // Implementation tasks:
  // - Connect to NATS server with TLS support
  // - Create JetStream context
  // - Implement Publish with retry logic
  // - Implement Subscribe with consumer groups
  // - Handle dead-letter queue
  // - Graceful shutdown
  ```

- [ ] **Tasks:**
  - [ ] Add NATS dependency to `go.mod`
    ```bash
    go get github.com/nats-io/nats.go@latest
    ```
  - [ ] Implement `NATSPublisher.Publish()`
    - [ ] Add circuit breaker wrapper
    - [ ] Add OpenTelemetry tracing spans
    - [ ] Add Prometheus metrics (publish_total, publish_duration)
    - [ ] Implement exponential backoff retry
  - [ ] Implement `NATSPublisher.PublishBatch()`
    - [ ] Batch size optimization (target: 100 msgs/batch)
    - [ ] Transaction support
  - [ ] Implement `NATSSubscriber.Subscribe()`
    - [ ] Consumer group support for HA
    - [ ] Manual ack for reliability
    - [ ] Error handling → dead-letter topic
  - [ ] Implement `Health()` check
    - [ ] Server connectivity test
    - [ ] JetStream availability check

#### MSG-1.3: Configuration Integration
**Day 4**

- [ ] **File:** Enhance `/internal/config/config.go`
  ```go
  type Config struct {
      // ... existing fields

      Messaging MessagingConfig `mapstructure:"messaging"`
  }

  type MessagingConfig struct {
      Enabled         bool     `mapstructure:"enabled"`
      Provider        string   `mapstructure:"provider"` // nats, kafka, rabbitmq, redis
      Brokers         []string `mapstructure:"brokers"`
      Username        string   `mapstructure:"username"`
      PasswordEnvVar  string   `mapstructure:"password_env_var"` // LUMO_MESSAGING_PASSWORD
      TLS             bool     `mapstructure:"tls"`
      EventTopic      string   `mapstructure:"event_topic"`
      DeadLetterTopic string   `mapstructure:"dead_letter_topic"`
      MaxRetries      int      `mapstructure:"max_retries"`
      RetryBackoff    string   `mapstructure:"retry_backoff"` // e.g., "5s"
  }
  ```

- [ ] **File:** Update `/configs/config.example.yaml`
  ```yaml
  messaging:
    enabled: true
    provider: nats  # nats, kafka, rabbitmq, redis
    brokers:
      - nats://localhost:4222
    username: ""
    password_env_var: LUMO_MESSAGING_PASSWORD
    tls: false
    event_topic: lumo.events
    dead_letter_topic: lumo.events.dlq
    max_retries: 3
    retry_backoff: 5s
  ```

- [ ] **Environment Variables:**
  ```bash
  export LUMO_MESSAGING_ENABLED=true
  export LUMO_MESSAGING_PROVIDER=nats
  export LUMO_MESSAGING_BROKERS=nats://localhost:4222
  export LUMO_MESSAGING_PASSWORD=secret
  ```

#### MSG-1.4: Unit Tests for NATS
**Day 5**

- [ ] **File:** Create `/internal/messaging/providers/nats_test.go`
- [ ] **Tests:**
  - [ ] `TestNATSPublisher_Publish()` - Success, failure, retry
  - [ ] `TestNATSPublisher_PublishBatch()` - Batch processing
  - [ ] `TestNATSSubscriber_Subscribe()` - Message receipt, ack
  - [ ] `TestNATSDeadLetterQueue()` - Failed message routing
  - [ ] `TestNATSReconnection()` - Connection loss recovery
  - [ ] `TestNATSTLS()` - Secure connection
- [ ] **Estimated LOC:** 300-350 lines
- [ ] **Pattern:** Use testcontainers for NATS server
  ```go
  import "github.com/testcontainers/testcontainers-go/modules/nats"

  func setupNATSContainer(t *testing.T) (*nats.NATSContainer, error) {
      ctx := context.Background()
      natsContainer, err := nats.RunContainer(ctx,
          testcontainers.WithImage("nats:2.10-alpine"),
      )
      // ... setup JetStream
  }
  ```

---

### Week 2: Kafka, RabbitMQ, Redis Providers

#### MSG-2.1: Kafka Provider
**Days 6-8**

- [ ] **File:** Create `/internal/messaging/providers/kafka.go`
  ```go
  package providers

  import (
      "github.com/segmentio/kafka-go"
      "github.com/ignacio/lumo/internal/messaging"
  )

  type KafkaProvider struct{}

  type KafkaPublisher struct {
      writer *kafka.Writer
      config *messaging.Config
  }

  type KafkaSubscriber struct {
      reader *kafka.Reader
      config *messaging.Config
  }
  ```

- [ ] **Tasks:**
  - [ ] Add Kafka dependency
    ```bash
    go get github.com/segmentio/kafka-go@latest
    ```
  - [ ] Implement `KafkaPublisher.Publish()`
    - [ ] Producer with compression (snappy)
    - [ ] Idempotent producer for exactly-once
    - [ ] Partition key support
  - [ ] Implement `KafkaPublisher.PublishBatch()`
    - [ ] Batch accumulation (target: 1MB or 100 msgs)
  - [ ] Implement `KafkaSubscriber.Subscribe()`
    - [ ] Consumer group membership
    - [ ] Offset management (manual commit)
    - [ ] Rebalancing handling
  - [ ] Implement dead-letter topic routing

- [ ] **File:** Create `/internal/messaging/providers/kafka_test.go`
- [ ] **Estimated LOC:** 400 lines implementation + 350 test lines

#### MSG-2.2: RabbitMQ Provider
**Days 8-9**

- [ ] **File:** Create `/internal/messaging/providers/rabbitmq.go`
  ```go
  package providers

  import (
      "github.com/rabbitmq/amqp091-go"
      "github.com/ignacio/lumo/internal/messaging"
  )

  type RabbitMQProvider struct{}

  type RabbitMQPublisher struct {
      conn    *amqp091.Connection
      channel *amqp091.Channel
      config  *messaging.Config
  }

  type RabbitMQSubscriber struct {
      conn    *amqp091.Connection
      channel *amqp091.Channel
      config  *messaging.Config
  }
  ```

- [ ] **Tasks:**
  - [ ] Add RabbitMQ dependency
    ```bash
    go get github.com/rabbitmq/amqp091-go@latest
    ```
  - [ ] Implement `RabbitMQPublisher.Publish()`
    - [ ] Exchange declaration (topic exchange)
    - [ ] Publisher confirms for reliability
    - [ ] Persistent messages
  - [ ] Implement `RabbitMQSubscriber.Subscribe()`
    - [ ] Queue declaration with TTL
    - [ ] Prefetch count for flow control
    - [ ] Manual ack/nack
  - [ ] Implement DLX (dead-letter exchange)

- [ ] **File:** Create `/internal/messaging/providers/rabbitmq_test.go`
- [ ] **Estimated LOC:** 350 lines implementation + 300 test lines

#### MSG-2.3: Redis Streams Provider
**Day 10**

- [ ] **File:** Create `/internal/messaging/providers/redis_streams.go`
  ```go
  package providers

  import (
      "github.com/go-redis/redis/v9"
      "github.com/ignacio/lumo/internal/messaging"
  )

  type RedisProvider struct{}

  type RedisPublisher struct {
      client *redis.Client
      config *messaging.Config
  }

  type RedisSubscriber struct {
      client *redis.Client
      config *messaging.Config
  }
  ```

- [ ] **Tasks:**
  - [ ] Implement `RedisPublisher.Publish()`
    - [ ] XADD command for stream append
    - [ ] Stream trimming (MAXLEN ~1000)
  - [ ] Implement `RedisSubscriber.Subscribe()`
    - [ ] Consumer group (XGROUP CREATE)
    - [ ] XREADGROUP for consumption
    - [ ] XACK for acknowledgment
  - [ ] Implement pending entries recovery (XPENDING)

- [ ] **File:** Create `/internal/messaging/providers/redis_streams_test.go`
- [ ] **Estimated LOC:** 250 lines implementation + 200 test lines

---

### Week 3: Integration, Testing, Documentation

#### MSG-3.1: Agent Integration
**Days 11-12**

- [ ] **File:** Enhance `/internal/agent/eventdriven/api_processor.go`
  ```go
  package eventdriven

  import (
      "github.com/ignacio/lumo/internal/messaging"
  )

  type APIProcessor struct {
      // ... existing fields
      publisher messaging.Publisher // NEW: Replace HTTP client
      config    *messaging.Config
  }

  func (p *APIProcessor) submitEvent(event *types.Event) error {
      // OLD: HTTP POST to API server
      // NEW: Publish to message queue

      data, err := json.Marshal(event)
      if err != nil {
          return fmt.Errorf("marshal event: %w", err)
      }

      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()

      err = p.publisher.Publish(ctx, p.config.EventTopic, data)
      if err != nil {
          return fmt.Errorf("publish event: %w", err)
      }

      // Update metrics
      eventPublishTotal.WithLabelValues(event.Type, "success").Inc()
      return nil
  }
  ```

- [ ] **Tasks:**
  - [ ] Update `NewAPIProcessor()` to accept messaging publisher
  - [ ] Add fallback to HTTP if messaging disabled
  - [ ] Update OpenTelemetry spans for message publish
  - [ ] Update Prometheus metrics

#### MSG-3.2: API Server Integration
**Days 12-13**

- [ ] **File:** Create `/internal/api/consumers/event_consumer.go`
  ```go
  package consumers

  import (
      "github.com/ignacio/lumo/internal/messaging"
      "github.com/ignacio/lumo/internal/api/handlers"
  )

  type EventConsumer struct {
      subscriber messaging.Subscriber
      handler    *handlers.EventHandler
      config     *messaging.Config
  }

  func NewEventConsumer(sub messaging.Subscriber, handler *handlers.EventHandler) *EventConsumer {
      return &EventConsumer{
          subscriber: sub,
          handler:    handler,
      }
  }

  func (c *EventConsumer) Start(ctx context.Context) error {
      return c.subscriber.Subscribe(ctx, c.config.EventTopic, c.handleMessage)
  }

  func (c *EventConsumer) handleMessage(ctx context.Context, msg *messaging.Message) error {
      var event types.Event
      if err := json.Unmarshal(msg.Data, &event); err != nil {
          return fmt.Errorf("unmarshal event: %w", err)
      }

      // Process event (AI analysis, notifications, storage)
      return c.handler.ProcessEvent(ctx, &event)
  }
  ```

- [ ] **File:** Update `/cmd/lumo/serve.go`
  ```go
  // Start event consumer
  if cfg.Messaging.Enabled {
      provider, err := messaging.NewProvider(&cfg.Messaging)
      if err != nil {
          log.Fatalf("Failed to create messaging provider: %v", err)
      }

      subscriber, err := provider.NewSubscriber(&cfg.Messaging)
      if err != nil {
          log.Fatalf("Failed to create subscriber: %v", err)
      }

      consumer := consumers.NewEventConsumer(subscriber, eventHandler)
      go func() {
          if err := consumer.Start(context.Background()); err != nil {
              log.Errorf("Event consumer failed: %v", err)
          }
      }()
  }
  ```

#### MSG-3.3: Integration Tests
**Day 14**

- [ ] **File:** Create `/tests/integration/messaging_test.go`
- [ ] **Tests:**
  - [ ] `TestNATSEndToEnd()` - Agent publish → API consume
  - [ ] `TestKafkaEndToEnd()` - Event flow through Kafka
  - [ ] `TestRabbitMQEndToEnd()` - Event flow through RabbitMQ
  - [ ] `TestRedisStreamsEndToEnd()` - Event flow through Redis
  - [ ] `TestDeadLetterQueue()` - Failed message routing
  - [ ] `TestMessageOrdering()` - Event sequence preservation
  - [ ] `TestHighThroughput()` - 1,000 events/sec test
- [ ] **Estimated LOC:** 500-600 lines
- [ ] **Pattern:** Use testcontainers for all brokers

#### MSG-3.4: Load Testing
**Day 15**

- [ ] **File:** Create `/tests/load/messaging_load_test.go`
- [ ] **Scenarios:**
  - [ ] Sustained load: 1,000 events/sec for 5 minutes
  - [ ] Burst load: 5,000 events/sec for 30 seconds
  - [ ] Concurrent publishers: 50 agents publishing simultaneously
  - [ ] Consumer lag: Measure processing delay under load
- [ ] **Success Criteria:**
  - [ ] <10ms publish latency (p99)
  - [ ] <100ms end-to-end latency (publish → process)
  - [ ] Zero message loss
  - [ ] Consumer keeps up with producer

#### MSG-3.5: Documentation
**Day 15**

- [ ] **File:** Create `/internal/messaging/README.md`
  ```markdown
  # Messaging System

  ## Overview
  Pub/sub framework supporting NATS, Kafka, RabbitMQ, Redis Streams.

  ## Provider Comparison
  | Provider | Best For | Throughput | Latency | Features |
  |----------|----------|------------|---------|----------|
  | NATS | Low latency, simplicity | High | <1ms | JetStream, KV |
  | Kafka | High throughput, durability | Very High | ~5ms | Replay, partitions |
  | RabbitMQ | Complex routing, enterprise | Medium | ~3ms | DLX, routing keys |
  | Redis | Existing Redis infra | Medium | <2ms | Simple, in-memory |

  ## Configuration
  [Example configurations for each provider]

  ## Usage Examples
  [Code examples for publishing and subscribing]

  ## Troubleshooting
  [Common issues and solutions]
  ```

- [ ] **File:** Update `/deployments/kubernetes/base/configmap-agent.yaml`
  - [ ] Add messaging configuration section
  - [ ] Add NATS deployment example

- [ ] **File:** Create `/deployments/kubernetes/nats/deployment.yaml`
  ```yaml
  # NATS JetStream deployment for K8s
  # StatefulSet with persistent volumes
  # Service configuration
  ```

---

## Phase 14: Advanced Reporting

**Priority:** #2 📊 **MARKET DIFFERENTIATOR**
**Timeline:** 2-3 weeks (can run parallel with Phase 11c)
**Estimated LOC:** ~2,000 lines + 1,000 test lines
**Dependencies:** None

**Success Metrics:**
- [ ] <5 sec report generation for 30-day reports
- [ ] PDF, HTML, CSV exports working
- [ ] Trend detection accuracy >90%
- [ ] Scheduled report delivery functional

---

### Week 1: Database Schema + Report Engine

#### RPT-1.1: Database Schema Extensions
**Days 1-2**

- [ ] **File:** Create `/internal/database/migrations/015_reports_schema.sql`
  ```sql
  -- Reports metadata table
  CREATE TABLE IF NOT EXISTS reports (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      name VARCHAR(255) NOT NULL,
      description TEXT,
      type VARCHAR(50) NOT NULL, -- system, custom, scheduled
      format VARCHAR(20) NOT NULL, -- pdf, html, csv, json
      created_by VARCHAR(255),
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      config JSONB NOT NULL, -- Report configuration
      CONSTRAINT reports_type_check CHECK (type IN ('system', 'custom', 'scheduled')),
      CONSTRAINT reports_format_check CHECK (format IN ('pdf', 'html', 'csv', 'json'))
  );

  CREATE INDEX idx_reports_created_at ON reports(created_at);
  CREATE INDEX idx_reports_type ON reports(type);

  -- Report schedules table
  CREATE TABLE IF NOT EXISTS report_schedules (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      report_id UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
      cron_expression VARCHAR(100) NOT NULL,
      enabled BOOLEAN DEFAULT true,
      last_run TIMESTAMP WITH TIME ZONE,
      next_run TIMESTAMP WITH TIME ZONE,
      recipients JSONB NOT NULL, -- Array of email addresses or notification channels
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
  );

  CREATE INDEX idx_report_schedules_next_run ON report_schedules(next_run) WHERE enabled = true;

  -- Metrics history table (for trend analysis)
  CREATE TABLE IF NOT EXISTS metrics_history (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
      metric_name VARCHAR(100) NOT NULL,
      metric_value DOUBLE PRECISION NOT NULL,
      metric_unit VARCHAR(50),
      tags JSONB, -- Additional metadata
      timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
  );

  CREATE INDEX idx_metrics_history_timestamp ON metrics_history(timestamp DESC);
  CREATE INDEX idx_metrics_history_agent_metric ON metrics_history(agent_id, metric_name, timestamp DESC);
  CREATE INDEX idx_metrics_history_tags ON metrics_history USING GIN(tags);

  -- Report executions table (audit trail)
  CREATE TABLE IF NOT EXISTS report_executions (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      report_id UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
      schedule_id UUID REFERENCES report_schedules(id) ON DELETE SET NULL,
      status VARCHAR(50) NOT NULL, -- pending, running, completed, failed
      started_at TIMESTAMP WITH TIME ZONE,
      completed_at TIMESTAMP WITH TIME ZONE,
      duration_ms INTEGER,
      output_path TEXT,
      error_message TEXT,
      metadata JSONB,
      CONSTRAINT report_executions_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed'))
  );

  CREATE INDEX idx_report_executions_report_id ON report_executions(report_id, completed_at DESC);
  CREATE INDEX idx_report_executions_status ON report_executions(status);
  ```

- [ ] **Run Migration:**
  ```bash
  cd internal/database/migrations
  goose postgres "postgres://user:pass@localhost/lumo" up
  ```

#### RPT-1.2: Repository Layer
**Day 2**

- [ ] **File:** Create `/internal/database/repository/report.go`
  ```go
  package repository

  type ReportRepository struct {
      db *sql.DB
  }

  type Report struct {
      ID          string
      Name        string
      Description string
      Type        string
      Format      string
      CreatedBy   string
      CreatedAt   time.Time
      UpdatedAt   time.Time
      Config      map[string]interface{}
  }

  func NewReportRepository(db *sql.DB) *ReportRepository {
      return &ReportRepository{db: db}
  }

  func (r *ReportRepository) Create(report *Report) error
  func (r *ReportRepository) Get(id string) (*Report, error)
  func (r *ReportRepository) List(filters ReportFilters) ([]*Report, error)
  func (r *ReportRepository) Update(report *Report) error
  func (r *ReportRepository) Delete(id string) error
  func (r *ReportRepository) CreateExecution(execution *ReportExecution) error
  func (r *ReportRepository) UpdateExecution(execution *ReportExecution) error
  func (r *ReportRepository) GetScheduledReports() ([]*ReportSchedule, error)
  ```

- [ ] **File:** Create `/internal/database/repository/metrics_history.go`
  ```go
  package repository

  type MetricsHistoryRepository struct {
      db *sql.DB
  }

  type MetricDataPoint struct {
      AgentID     string
      MetricName  string
      MetricValue float64
      MetricUnit  string
      Tags        map[string]string
      Timestamp   time.Time
  }

  func (r *MetricsHistoryRepository) Insert(datapoint *MetricDataPoint) error
  func (r *MetricsHistoryRepository) InsertBatch(datapoints []*MetricDataPoint) error
  func (r *MetricsHistoryRepository) Query(filters MetricsFilters) ([]*MetricDataPoint, error)
  func (r *MetricsHistoryRepository) Aggregate(filters MetricsFilters, aggregation string) (map[string]float64, error)
  func (r *MetricsHistoryRepository) GetTimeSeries(agentID, metricName string, start, end time.Time, interval string) ([]*TimeSeriesPoint, error)
  ```

#### RPT-1.3: Report Engine Core
**Days 3-4**

- [ ] **File:** Create `/internal/reporting/engine.go`
  ```go
  package reporting

  import (
      "context"
      "github.com/ignacio/lumo/internal/database/repository"
  )

  type Engine struct {
      reportRepo    *repository.ReportRepository
      metricsRepo   *repository.MetricsHistoryRepository
      eventRepo     *repository.EventRepository
      templateMgr   *TemplateManager
      exporters     map[string]Exporter
  }

  func NewEngine(
      reportRepo *repository.ReportRepository,
      metricsRepo *repository.MetricsHistoryRepository,
      eventRepo *repository.EventRepository,
  ) *Engine {
      engine := &Engine{
          reportRepo:  reportRepo,
          metricsRepo: metricsRepo,
          eventRepo:   eventRepo,
          exporters:   make(map[string]Exporter),
      }

      // Register exporters
      engine.RegisterExporter("pdf", NewPDFExporter())
      engine.RegisterExporter("html", NewHTMLExporter())
      engine.RegisterExporter("csv", NewCSVExporter())
      engine.RegisterExporter("json", NewJSONExporter())

      return engine
  }

  func (e *Engine) GenerateReport(ctx context.Context, reportID string, params ReportParams) (*ReportOutput, error)
  func (e *Engine) RegisterExporter(format string, exporter Exporter)
  func (e *Engine) GetExporter(format string) (Exporter, error)
  ```

- [ ] **File:** Create `/internal/reporting/types.go`
  ```go
  package reporting

  type ReportParams struct {
      StartTime   time.Time
      EndTime     time.Time
      AgentIDs    []string
      IncludeTrends bool
      IncludeAnomalies bool
      Filters     map[string]interface{}
  }

  type ReportOutput struct {
      Data      *ReportData
      Format    string
      Content   []byte
      Metadata  map[string]interface{}
      GeneratedAt time.Time
  }

  type ReportData struct {
      Summary         *ReportSummary
      Metrics         []*MetricSection
      Events          []*EventSection
      Trends          []*TrendAnalysis
      Anomalies       []*AnomalyDetection
      Recommendations []string
  }

  type ReportSummary struct {
      TotalAgents     int
      TotalEvents     int
      CriticalEvents  int
      AverageUptime   float64
      HealthScore     float64
  }
  ```

#### RPT-1.4: Data Collection
**Day 5**

- [ ] **File:** Create `/internal/reporting/collectors/metrics_collector.go`
  ```go
  package collectors

  type MetricsCollector struct {
      metricsRepo *repository.MetricsHistoryRepository
  }

  func (c *MetricsCollector) CollectSystemMetrics(agentIDs []string, start, end time.Time) (*MetricsData, error) {
      // Aggregate CPU, memory, disk metrics
      // Calculate averages, min, max, p95, p99
      // Group by agent and time window
  }

  func (c *MetricsCollector) CollectResourceUtilization(agentIDs []string, start, end time.Time) (*ResourceUtilization, error)
  ```

- [ ] **File:** Create `/internal/reporting/collectors/event_collector.go`
  ```go
  package collectors

  type EventCollector struct {
      eventRepo *repository.EventRepository
  }

  func (c *EventCollector) CollectEvents(filters EventFilters) (*EventData, error) {
      // Query events by severity, type, time range
      // Group by type and severity
      // Calculate event frequency
  }

  func (c *EventCollector) CollectIncidents(filters EventFilters) ([]*Incident, error) {
      // Group related events into incidents
      // Calculate MTTR, MTTD
  }
  ```

---

### Week 2: Templates + Exporters

#### RPT-2.1: Template System
**Days 6-7**

- [ ] **File:** Create `/internal/reporting/templates/manager.go`
  ```go
  package templates

  import "html/template"

  type Manager struct {
      templates map[string]*template.Template
  }

  func NewManager() *Manager {
      mgr := &Manager{
          templates: make(map[string]*template.Template),
      }
      mgr.loadBuiltinTemplates()
      return mgr
  }

  func (m *Manager) loadBuiltinTemplates() {
      // Load embedded templates
      m.Register("system_health", systemHealthTemplate)
      m.Register("executive_summary", executiveSummaryTemplate)
      m.Register("detailed_metrics", detailedMetricsTemplate)
      m.Register("incident_report", incidentReportTemplate)
  }

  func (m *Manager) Register(name string, tmpl *template.Template)
  func (m *Manager) Get(name string) (*template.Template, error)
  func (m *Manager) Render(name string, data interface{}) (string, error)
  ```

- [ ] **File:** Create `/internal/reporting/templates/system_health.html`
  ```html
  <!DOCTYPE html>
  <html>
  <head>
      <title>System Health Report</title>
      <style>
          /* Embedded CSS for styling */
      </style>
  </head>
  <body>
      <h1>System Health Report</h1>
      <div class="summary">
          <h2>Summary</h2>
          <p>Time Range: {{.StartTime}} - {{.EndTime}}</p>
          <p>Total Agents: {{.Summary.TotalAgents}}</p>
          <p>Health Score: {{.Summary.HealthScore}}%</p>
      </div>

      <div class="metrics">
          <h2>Metrics Overview</h2>
          {{range .Metrics}}
          <div class="metric-section">
              <h3>{{.Name}}</h3>
              <table>
                  <tr><th>Agent</th><th>Average</th><th>Peak</th><th>Trend</th></tr>
                  {{range .Data}}
                  <tr>
                      <td>{{.AgentName}}</td>
                      <td>{{.Average}}</td>
                      <td>{{.Peak}}</td>
                      <td>{{.Trend}}</td>
                  </tr>
                  {{end}}
              </table>
          </div>
          {{end}}
      </div>

      <div class="events">
          <h2>Events</h2>
          <!-- Event tables grouped by severity -->
      </div>

      {{if .Trends}}
      <div class="trends">
          <h2>Trend Analysis</h2>
          <!-- Trend charts and analysis -->
      </div>
      {{end}}

      <div class="footer">
          <p>Generated by Lumo at {{.GeneratedAt}}</p>
      </div>
  </body>
  </html>
  ```

- [ ] **Create Additional Templates:**
  - [ ] `/internal/reporting/templates/executive_summary.html` - High-level overview for executives
  - [ ] `/internal/reporting/templates/detailed_metrics.html` - Full metrics breakdown
  - [ ] `/internal/reporting/templates/incident_report.html` - Incident timeline and analysis

#### RPT-2.2: PDF Exporter
**Days 7-8**

- [ ] **File:** Create `/internal/reporting/exporters/pdf.go`
  ```go
  package exporters

  import "github.com/go-pdf/fpdf"

  type PDFExporter struct {
      templateMgr *templates.Manager
  }

  func NewPDFExporter() *PDFExporter {
      return &PDFExporter{}
  }

  func (e *PDFExporter) Export(data *ReportData, params ExportParams) ([]byte, error) {
      // Render HTML template
      html, err := e.templateMgr.Render(params.TemplateName, data)
      if err != nil {
          return nil, err
      }

      // Convert HTML to PDF
      pdf := fpdf.New("P", "mm", "A4", "")
      pdf.AddPage()

      // Add header
      e.addHeader(pdf, data.Summary)

      // Add sections
      e.addMetricsSection(pdf, data.Metrics)
      e.addEventsSection(pdf, data.Events)

      if len(data.Trends) > 0 {
          e.addTrendsSection(pdf, data.Trends)
      }

      // Add footer
      e.addFooter(pdf, data.GeneratedAt)

      // Output to buffer
      var buf bytes.Buffer
      err = pdf.Output(&buf)
      return buf.Bytes(), err
  }

  func (e *PDFExporter) addHeader(pdf *fpdf.Fpdf, summary *ReportSummary)
  func (e *PDFExporter) addMetricsSection(pdf *fpdf.Fpdf, metrics []*MetricSection)
  func (e *PDFExporter) addEventsSection(pdf *fpdf.Fpdf, events []*EventSection)
  func (e *PDFExporter) addTrendsSection(pdf *fpdf.Fpdf, trends []*TrendAnalysis)
  func (e *PDFExporter) addFooter(pdf *fpdf.Fpdf, timestamp time.Time)
  ```

- [ ] **Add Dependency:**
  ```bash
  go get github.com/go-pdf/fpdf@latest
  ```

#### RPT-2.3: HTML/CSV/JSON Exporters
**Day 8**

- [ ] **File:** Create `/internal/reporting/exporters/html.go`
  ```go
  func (e *HTMLExporter) Export(data *ReportData, params ExportParams) ([]byte, error) {
      // Render template with data
      // Add CSS styling
      // Return raw HTML
  }
  ```

- [ ] **File:** Create `/internal/reporting/exporters/csv.go`
  ```go
  func (e *CSVExporter) Export(data *ReportData, params ExportParams) ([]byte, error) {
      // Flatten nested data structures
      // Write CSV with headers
      // Support multiple sheets (metrics, events, trends)
  }
  ```

- [ ] **File:** Create `/internal/reporting/exporters/json.go`
  ```go
  func (e *JSONExporter) Export(data *ReportData, params ExportParams) ([]byte, error) {
      // Pretty-print JSON
      // Include all nested structures
      return json.MarshalIndent(data, "", "  ")
  }
  ```

#### RPT-2.4: Trend Detection
**Days 9-10**

- [ ] **File:** Create `/internal/reporting/analysis/trends.go`
  ```go
  package analysis

  type TrendAnalyzer struct {
      metricsRepo *repository.MetricsHistoryRepository
  }

  type TrendAnalysis struct {
      MetricName  string
      Direction   string // increasing, decreasing, stable, volatile
      Slope       float64
      Confidence  float64
      Prediction  *Prediction
  }

  type Prediction struct {
      NextValue   float64
      TimeToThreshold time.Duration
      Confidence  float64
  }

  func (a *TrendAnalyzer) Analyze(metricName string, agentID string, start, end time.Time) (*TrendAnalysis, error) {
      // Fetch time-series data
      data, err := a.metricsRepo.GetTimeSeries(agentID, metricName, start, end, "1h")
      if err != nil {
          return nil, err
      }

      // Calculate linear regression
      slope, intercept := a.linearRegression(data)

      // Determine trend direction
      direction := a.classifyTrend(slope, data)

      // Calculate confidence (R²)
      confidence := a.calculateRSquared(data, slope, intercept)

      // Predict next value
      prediction := a.predict(slope, intercept, data)

      return &TrendAnalysis{
          MetricName:  metricName,
          Direction:   direction,
          Slope:       slope,
          Confidence:  confidence,
          Prediction:  prediction,
      }, nil
  }

  func (a *TrendAnalyzer) linearRegression(data []*TimeSeriesPoint) (slope, intercept float64) {
      // Simple linear regression algorithm
      // y = mx + b
      // Use least squares method
  }

  func (a *TrendAnalyzer) classifyTrend(slope float64, data []*TimeSeriesPoint) string {
      // increasing: slope > threshold && consistent
      // decreasing: slope < -threshold && consistent
      // stable: abs(slope) < threshold
      // volatile: high variance
  }

  func (a *TrendAnalyzer) calculateRSquared(data []*TimeSeriesPoint, slope, intercept float64) float64 {
      // Calculate coefficient of determination
      // R² = 1 - (SS_res / SS_tot)
  }

  func (a *TrendAnalyzer) predict(slope, intercept float64, data []*TimeSeriesPoint) *Prediction {
      // Predict next value based on trend
      // Calculate time to threshold crossing
      // Estimate confidence based on R²
  }
  ```

---

### Week 3: CLI, Scheduling, Testing

#### RPT-3.1: CLI Command
**Days 11-12**

- [ ] **File:** Create `/cmd/lumo/report.go`
  ```go
  package main

  var reportCmd = &cobra.Command{
      Use:   "report",
      Short: "Generate and manage reports",
      Long:  `Generate system health reports, metrics analysis, and incident reports.`,
  }

  var reportGenerateCmd = &cobra.Command{
      Use:   "generate [report-name]",
      Short: "Generate a report",
      Example: `  lumo report generate system-health --start 2025-11-01 --end 2025-11-30 --format pdf
    lumo report generate executive-summary --last 7d --format html
    lumo report generate incidents --severity critical --format csv`,
      Run: func(cmd *cobra.Command, args []string) {
          // Parse flags
          start, _ := cmd.Flags().GetString("start")
          end, _ := cmd.Flags().GetString("end")
          format, _ := cmd.Flags().GetString("format")
          output, _ := cmd.Flags().GetString("output")
          agentIDs, _ := cmd.Flags().GetStringSlice("agents")
          includeTrends, _ := cmd.Flags().GetBool("trends")

          // Generate report
          engine := reporting.NewEngine(...)
          report, err := engine.GenerateReport(ctx, reportID, params)
          if err != nil {
              log.Fatalf("Failed to generate report: %v", err)
          }

          // Save to file or stdout
          if output == "-" {
              os.Stdout.Write(report.Content)
          } else {
              ioutil.WriteFile(output, report.Content, 0644)
          }
      },
  }

  var reportListCmd = &cobra.Command{
      Use:   "list",
      Short: "List available reports",
      Run: func(cmd *cobra.Command, args []string) {
          // List reports from database
      },
  }

  var reportScheduleCmd = &cobra.Command{
      Use:   "schedule [report-name]",
      Short: "Schedule a report for periodic generation",
      Example: `  lumo report schedule system-health --cron "0 9 * * MON" --recipients admin@example.com`,
      Run: func(cmd *cobra.Command, args []string) {
          // Create schedule in database
      },
  }

  func init() {
      reportCmd.AddCommand(reportGenerateCmd)
      reportCmd.AddCommand(reportListCmd)
      reportCmd.AddCommand(reportScheduleCmd)

      // Flags for generate
      reportGenerateCmd.Flags().String("start", "", "Start time (YYYY-MM-DD)")
      reportGenerateCmd.Flags().String("end", "", "End time (YYYY-MM-DD)")
      reportGenerateCmd.Flags().String("last", "", "Last duration (e.g., 7d, 24h, 30d)")
      reportGenerateCmd.Flags().String("format", "pdf", "Output format (pdf, html, csv, json)")
      reportGenerateCmd.Flags().String("output", "-", "Output file path (- for stdout)")
      reportGenerateCmd.Flags().StringSlice("agents", []string{}, "Filter by agent IDs")
      reportGenerateCmd.Flags().Bool("trends", true, "Include trend analysis")
      reportGenerateCmd.Flags().Bool("anomalies", false, "Include anomaly detection")

      // Flags for schedule
      reportScheduleCmd.Flags().String("cron", "", "Cron expression (required)")
      reportScheduleCmd.Flags().StringSlice("recipients", []string{}, "Email recipients")
      reportScheduleCmd.MarkFlagRequired("cron")
      reportScheduleCmd.MarkFlagRequired("recipients")

      rootCmd.AddCommand(reportCmd)
  }
  ```

#### RPT-3.2: Report Scheduler
**Day 12**

- [ ] **File:** Create `/internal/reporting/scheduler.go`
  ```go
  package reporting

  import "github.com/robfig/cron/v3"

  type Scheduler struct {
      cron        *cron.Cron
      engine      *Engine
      reportRepo  *repository.ReportRepository
      notifier    *notifications.Manager
  }

  func NewScheduler(engine *Engine, reportRepo *repository.ReportRepository, notifier *notifications.Manager) *Scheduler {
      return &Scheduler{
          cron:       cron.New(),
          engine:     engine,
          reportRepo: reportRepo,
          notifier:   notifier,
      }
  }

  func (s *Scheduler) Start(ctx context.Context) error {
      // Load scheduled reports from database
      schedules, err := s.reportRepo.GetScheduledReports()
      if err != nil {
          return fmt.Errorf("load schedules: %w", err)
      }

      // Register cron jobs
      for _, schedule := range schedules {
          if !schedule.Enabled {
              continue
          }

          _, err := s.cron.AddFunc(schedule.CronExpression, func() {
              s.executeScheduledReport(schedule)
          })
          if err != nil {
              log.Errorf("Failed to schedule report %s: %v", schedule.ReportID, err)
          }
      }

      s.cron.Start()
      <-ctx.Done()
      s.cron.Stop()
      return nil
  }

  func (s *Scheduler) executeScheduledReport(schedule *ReportSchedule) {
      // Generate report
      report, err := s.engine.GenerateReport(ctx, schedule.ReportID, schedule.Params)
      if err != nil {
          log.Errorf("Failed to generate scheduled report: %v", err)
          return
      }

      // Send to recipients
      for _, recipient := range schedule.Recipients {
          err := s.notifier.SendReport(recipient, report)
          if err != nil {
              log.Errorf("Failed to send report to %s: %v", recipient, err)
          }
      }

      // Update last_run timestamp
      s.reportRepo.UpdateScheduleLastRun(schedule.ID, time.Now())
  }
  ```

- [ ] **File:** Update `/cmd/lumo/serve.go` to start scheduler
  ```go
  // Start report scheduler
  scheduler := reporting.NewScheduler(reportEngine, reportRepo, notificationManager)
  go func() {
      if err := scheduler.Start(ctx); err != nil {
          log.Errorf("Report scheduler failed: %v", err)
      }
  }()
  ```

#### RPT-3.3: Tests
**Days 13-14**

- [ ] **File:** Create `/internal/reporting/engine_test.go`
- [ ] **Tests:**
  - [ ] `TestEngine_GenerateReport()` - Complete report generation
  - [ ] `TestEngine_ExportPDF()` - PDF export functionality
  - [ ] `TestEngine_ExportHTML()` - HTML export functionality
  - [ ] `TestEngine_ExportCSV()` - CSV export functionality
  - [ ] `TestTrendAnalyzer_LinearRegression()` - Regression algorithm
  - [ ] `TestTrendAnalyzer_ClassifyTrend()` - Trend classification
  - [ ] `TestScheduler_ExecuteReport()` - Scheduled report execution
- [ ] **Estimated LOC:** 600-700 lines

- [ ] **File:** Create `/internal/reporting/integration_test.go`
- [ ] **Tests:**
  - [ ] `TestEndToEnd_SystemHealthReport()` - Full workflow
  - [ ] `TestEndToEnd_ExecutiveSummary()` - Executive report
  - [ ] `TestEndToEnd_ScheduledDelivery()` - Email delivery
- [ ] **Estimated LOC:** 300-400 lines

#### RPT-3.4: Documentation
**Day 15**

- [ ] **File:** Create `/internal/reporting/README.md`
  ```markdown
  # Reporting System

  ## Overview
  Advanced reporting with PDF/HTML/CSV exports, trend analysis, and scheduled delivery.

  ## Report Types
  1. System Health - Overall infrastructure health
  2. Executive Summary - High-level metrics for stakeholders
  3. Detailed Metrics - Full metrics breakdown
  4. Incident Reports - Event timeline and analysis

  ## Usage Examples
  [CLI examples for each report type]

  ## Templates
  [How to create custom templates]

  ## Scheduling
  [Cron expression examples]
  ```

---

## Phase 17a: Multi-Cluster Orchestration

**Priority:** #3 🌐 **ENTERPRISE REQUIREMENT**
**Timeline:** 3-4 weeks
**Estimated LOC:** ~2,500 lines + 1,200 test lines
**Dependencies:** Phase 11c (messaging) recommended

**Success Metrics:**
- [ ] Manage 10+ clusters from single control plane
- [ ] <30s cross-cluster failover
- [ ] Unified alerting across clusters
- [ ] 99.9% control plane availability

---

### Week 1-2: Federation API + Agent Discovery

#### MC-1.1: Cluster Registry
**Days 1-3**

- [ ] **File:** Create `/internal/database/migrations/016_multi_cluster_schema.sql`
  ```sql
  CREATE TABLE IF NOT EXISTS clusters (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      name VARCHAR(255) NOT NULL UNIQUE,
      description TEXT,
      kubeconfig TEXT, -- Encrypted kubeconfig
      api_endpoint VARCHAR(255) NOT NULL,
      status VARCHAR(50) NOT NULL, -- healthy, degraded, unreachable
      version VARCHAR(50),
      region VARCHAR(100),
      environment VARCHAR(50), -- dev, staging, production
      tags JSONB,
      last_seen TIMESTAMP WITH TIME ZONE,
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      CONSTRAINT clusters_status_check CHECK (status IN ('healthy', 'degraded', 'unreachable'))
  );

  CREATE INDEX idx_clusters_status ON clusters(status);
  CREATE INDEX idx_clusters_region ON clusters(region);

  -- Update agents table to include cluster reference
  ALTER TABLE agents ADD COLUMN cluster_id UUID REFERENCES clusters(id) ON DELETE CASCADE;
  CREATE INDEX idx_agents_cluster_id ON agents(cluster_id);

  -- Multi-cluster events
  ALTER TABLE events ADD COLUMN cluster_id UUID REFERENCES clusters(id) ON DELETE CASCADE;
  CREATE INDEX idx_events_cluster_id ON events(cluster_id);
  ```

- [ ] **File:** Create `/internal/database/repository/cluster.go`
  ```go
  package repository

  type ClusterRepository struct {
      db *sql.DB
  }

  type Cluster struct {
      ID          string
      Name        string
      Description string
      APIEndpoint string
      Status      string
      Version     string
      Region      string
      Environment string
      Tags        map[string]string
      LastSeen    time.Time
      CreatedAt   time.Time
      UpdatedAt   time.Time
  }

  func (r *ClusterRepository) Create(cluster *Cluster) error
  func (r *ClusterRepository) Get(id string) (*Cluster, error)
  func (r *ClusterRepository) GetByName(name string) (*Cluster, error)
  func (r *ClusterRepository) List(filters ClusterFilters) ([]*Cluster, error)
  func (r *ClusterRepository) Update(cluster *Cluster) error
  func (r *ClusterRepository) Delete(id string) error
  func (r *ClusterRepository) UpdateStatus(id, status string) error
  func (r *ClusterRepository) UpdateHeartbeat(id string) error
  ```

#### MC-1.2: Federation API
**Days 4-6**

- [ ] **File:** Create `/internal/api/handlers/clusters.go`
  ```go
  package handlers

  type ClusterHandler struct {
      clusterRepo *repository.ClusterRepository
      agentRepo   *repository.AgentRepository
      k8sClient   *kubernetes.Clientset
  }

  // POST /api/v1/clusters - Register new cluster
  func (h *ClusterHandler) RegisterCluster(w http.ResponseWriter, r *http.Request) {
      // Validate cluster connectivity
      // Store cluster metadata
      // Return cluster ID and auth token
  }

  // GET /api/v1/clusters - List all clusters
  func (h *ClusterHandler) ListClusters(w http.ResponseWriter, r *http.Request)

  // GET /api/v1/clusters/:id - Get cluster details
  func (h *ClusterHandler) GetCluster(w http.ResponseWriter, r *http.Request)

  // PUT /api/v1/clusters/:id - Update cluster metadata
  func (h *ClusterHandler) UpdateCluster(w http.ResponseWriter, r *http.Request)

  // DELETE /api/v1/clusters/:id - Deregister cluster
  func (h *ClusterHandler) DeleteCluster(w http.ResponseWriter, r *http.Request)

  // GET /api/v1/clusters/:id/agents - List agents in cluster
  func (h *ClusterHandler) ListClusterAgents(w http.ResponseWriter, r *http.Request)

  // GET /api/v1/clusters/:id/events - List events from cluster
  func (h *ClusterHandler) ListClusterEvents(w http.ResponseWriter, r *http.Request)

  // GET /api/v1/clusters/:id/health - Get cluster health status
  func (h *ClusterHandler) GetClusterHealth(w http.ResponseWriter, r *http.Request) {
      // Aggregate agent health
      // Check K8s API connectivity
      // Return overall cluster health
  }
  ```

#### MC-1.3: Cross-Cluster Agent Discovery
**Days 7-9**

- [ ] **File:** Create `/internal/multicluster/discovery.go`
  ```go
  package multicluster

  type DiscoveryService struct {
      clusterRepo *repository.ClusterRepository
      agentRepo   *repository.AgentRepository
      k8sClients  map[string]*kubernetes.Clientset
  }

  func NewDiscoveryService(clusterRepo *repository.ClusterRepository, agentRepo *repository.AgentRepository) *DiscoveryService {
      return &DiscoveryService{
          clusterRepo: clusterRepo,
          agentRepo:   agentRepo,
          k8sClients:  make(map[string]*kubernetes.Clientset),
      }
  }

  func (s *DiscoveryService) Start(ctx context.Context) error {
      ticker := time.NewTicker(30 * time.Second)
      defer ticker.Stop()

      for {
          select {
          case <-ticker.C:
              s.discoverClusters()
          case <-ctx.Done():
              return nil
          }
      }
  }

  func (s *DiscoveryService) discoverClusters() {
      clusters, err := s.clusterRepo.List(ClusterFilters{})
      if err != nil {
          log.Errorf("Failed to list clusters: %v", err)
          return
      }

      for _, cluster := range clusters {
          go s.syncCluster(cluster)
      }
  }

  func (s *DiscoveryService) syncCluster(cluster *Cluster) {
      // Connect to cluster K8s API
      client, err := s.getK8sClient(cluster)
      if err != nil {
          s.markClusterUnreachable(cluster.ID)
          return
      }

      // List lumo-agent pods in cluster
      pods, err := client.CoreV1().Pods("lumo-system").List(context.Background(), metav1.ListOptions{
          LabelSelector: "app=lumo-agent",
      })
      if err != nil {
          log.Errorf("Failed to list agents in cluster %s: %v", cluster.Name, err)
          return
      }

      // Update agent registry
      for _, pod := range pods.Items {
          s.registerAgent(cluster.ID, &pod)
      }

      // Update cluster status
      s.clusterRepo.UpdateStatus(cluster.ID, "healthy")
      s.clusterRepo.UpdateHeartbeat(cluster.ID)
  }

  func (s *DiscoveryService) getK8sClient(cluster *Cluster) (*kubernetes.Clientset, error) {
      // Check cache first
      if client, exists := s.k8sClients[cluster.ID]; exists {
          return client, nil
      }

      // Create new client from kubeconfig
      config, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
          return clientcmd.Load([]byte(cluster.Kubeconfig))
      })
      if err != nil {
          return nil, err
      }

      client, err := kubernetes.NewForConfig(config)
      if err != nil {
          return nil, err
      }

      s.k8sClients[cluster.ID] = client
      return client, nil
  }
  ```

---

### Week 3: Unified Alerting + Cross-Cluster Views

#### MC-2.1: Unified Event Aggregation
**Days 10-11**

- [ ] **File:** Create `/internal/multicluster/aggregator.go`
  ```go
  package multicluster

  type EventAggregator struct {
      eventRepo   *repository.EventRepository
      clusterRepo *repository.ClusterRepository
      notifier    *notifications.Manager
  }

  func (a *EventAggregator) AggregateEvents(filters EventFilters) (*AggregatedEvents, error) {
      // Query events across all clusters
      // Group by cluster, severity, type
      // Calculate cluster-level statistics
  }

  func (a *EventAggregator) GetClusterComparison() (*ClusterComparison, error) {
      // Compare health across clusters
      // Identify outlier clusters
      // Calculate cluster reliability scores
  }
  ```

#### MC-2.2: Cross-Cluster Notifications
**Days 11-12**

- [ ] **File:** Enhance `/internal/notifications/manager.go`
  ```go
  func (m *Manager) SendCrossClusterAlert(event *Event, clusters []*Cluster) error {
      // Send notification indicating which cluster(s) affected
      // Include cluster context in message
      // Support cluster-specific routing rules
  }
  ```

- [ ] **Message Templates:**
  - [ ] Update Slack template to include cluster name
  - [ ] Update Telegram template with cluster badges
  - [ ] Update Email template with cluster sections

#### MC-2.3: CLI Enhancements
**Days 13-14**

- [ ] **File:** Create `/cmd/lumo/cluster.go`
  ```go
  var clusterCmd = &cobra.Command{
      Use:   "cluster",
      Short: "Manage multi-cluster configuration",
  }

  var clusterListCmd = &cobra.Command{
      Use:   "list",
      Short: "List registered clusters",
      Run: func(cmd *cobra.Command, args []string) {
          // Call API to list clusters
          // Display table with status, region, agents
      },
  }

  var clusterRegisterCmd = &cobra.Command{
      Use:   "register [name]",
      Short: "Register a new cluster",
      Example: `  lumo cluster register prod-us-east --kubeconfig ~/.kube/config --region us-east-1`,
      Run: func(cmd *cobra.Command, args []string) {
          // Read kubeconfig
          // Validate cluster connectivity
          // Call API to register cluster
      },
  }

  var clusterHealthCmd = &cobra.Command{
      Use:   "health [cluster-name]",
      Short: "Get cluster health status",
      Run: func(cmd *cobra.Command, args []string) {
          // Call API to get cluster health
          // Display agent count, events, uptime
      },
  }

  var clusterEventsCmd = &cobra.Command{
      Use:   "events [cluster-name]",
      Short: "List events from cluster",
      Run: func(cmd *cobra.Command, args []string) {
          // Call API to list cluster events
          // Support filtering by severity, type
      },
  }
  ```

---

### Week 4: Testing + Documentation

#### MC-3.1: Integration Tests
**Days 15-17**

- [ ] **File:** Create `/tests/integration/multicluster_test.go`
- [ ] **Tests:**
  - [ ] `TestClusterRegistration()` - Register and deregister clusters
  - [ ] `TestCrossClusterDiscovery()` - Agent discovery across clusters
  - [ ] `TestUnifiedEventAggregation()` - Event querying
  - [ ] `TestCrossClusterNotifications()` - Alert routing
- [ ] **Estimated LOC:** 400-500 lines
- [ ] **Setup:** Use kind to create multiple test clusters

#### MC-3.2: Documentation
**Days 18-19**

- [ ] **File:** Create `/docs/multi-cluster-setup.md`
  ```markdown
  # Multi-Cluster Setup Guide

  ## Overview
  Manage multiple Kubernetes clusters from single Lumo control plane.

  ## Architecture
  [Diagram showing control plane and managed clusters]

  ## Prerequisites
  - Lumo API server running
  - Network connectivity to all cluster K8s APIs
  - Cluster admin access for each cluster

  ## Registration
  [Step-by-step cluster registration]

  ## Agent Deployment
  [Deploy agents in managed clusters]

  ## Monitoring
  [View cross-cluster events and health]

  ## Troubleshooting
  [Common issues and solutions]
  ```

---

## Test Coverage Enhancement

**Timeline:** 1-2 weeks (ongoing)
**Target:** 70%+ overall, 90%+ critical packages

### COV-1: Priority Package Coverage

#### COV-1.1: Database Repository Tests
- [ ] **File:** `/internal/database/repository/agent_test.go` (currently 0%)
- [ ] **Target:** 85%+ coverage
- [ ] **Estimated LOC:** 250 lines
- [ ] **Priority:** HIGH - Critical data layer

#### COV-1.2: API Handler Tests
- [ ] **File:** `/internal/api/handlers/diagnostics_test.go`
- [ ] **Expand:** Edge cases, error paths, validation
- [ ] **Target:** 90%+ coverage
- [ ] **Estimated LOC:** 200 additional lines

#### COV-1.3: Event-Driven Watchers
- [ ] **File:** `/internal/agent/eventdriven/watchers/pod_test.go`
- [ ] **Expand:** All event type triggers, edge cases
- [ ] **Target:** 85%+ coverage
- [ ] **Estimated LOC:** 300 additional lines

### COV-2: Coverage Tooling

- [ ] **File:** Create `/scripts/coverage-report.sh`
  ```bash
  #!/bin/bash
  # Generate coverage report with highlighting for low-coverage packages

  go test -coverprofile=coverage.out ./internal/...
  go tool cover -html=coverage.out -o coverage.html

  # Parse coverage and identify packages below threshold
  go tool cover -func=coverage.out | awk '{if ($3 < "70.0%") print $1, $3}'
  ```

- [ ] **CI Integration:** Add coverage check to GitHub Actions
  ```yaml
  - name: Test Coverage
    run: |
      make ci-test
      go tool cover -func=coverage.out | grep total | awk '{print $3}'
      # Fail if below 70%
  ```

---

## Performance Optimization

**Timeline:** 1 week
**Expected Gain:** 30-50% API latency reduction

### PERF-1: Database Query Optimization

#### PERF-1.1: Query Result Caching
**Days 1-2**

- [ ] **File:** Enhance `/internal/api/handlers/agents.go`
  ```go
  func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
      // Check Redis cache first
      cacheKey := fmt.Sprintf("agents:list:%s", hash(filters))
      cached, err := h.cache.Get(cacheKey)
      if err == nil {
          // Return cached result
          response.JSON(w, http.StatusOK, cached)
          return
      }

      // Query database
      agents, err := h.agentRepo.List(filters)
      if err != nil {
          response.Error(w, err)
          return
      }

      // Cache result (TTL: 30 seconds)
      h.cache.Set(cacheKey, agents, 30*time.Second)

      response.JSON(w, http.StatusOK, agents)
  }
  ```

- [ ] **Cache Invalidation:**
  - [ ] Invalidate on agent registration
  - [ ] Invalidate on heartbeat status change
  - [ ] Implement cache tags for fine-grained invalidation

#### PERF-1.2: Batch Database Inserts
**Day 2**

- [ ] **File:** Enhance `/internal/database/repository/event.go`
  ```go
  func (r *EventRepository) InsertBatch(events []*Event) error {
      // Use PostgreSQL COPY or multi-row INSERT
      // Batch size: 100 events
      // Transaction-based for consistency

      tx, err := r.db.Begin()
      if err != nil {
          return err
      }
      defer tx.Rollback()

      stmt, err := tx.Prepare(pq.CopyIn("events", "id", "type", "severity", ...))
      if err != nil {
          return err
      }

      for _, event := range events {
          _, err = stmt.Exec(event.ID, event.Type, event.Severity, ...)
          if err != nil {
              return err
          }
      }

      _, err = stmt.Exec()
      if err != nil {
          return err
      }

      return tx.Commit()
  }
  ```

### PERF-2: Redis Cache Tuning

#### PERF-2.1: Cache Hit Rate Analysis
**Day 3**

- [ ] **File:** Create `/scripts/analyze-cache-hits.sh`
  ```bash
  #!/bin/bash
  # Connect to Redis and analyze hit rates

  redis-cli INFO stats | grep keyspace
  redis-cli --stat

  # Calculate hit rate percentage
  # Identify cold cache keys
  ```

- [ ] **Optimization Tasks:**
  - [ ] Increase TTL for stable data (agent list)
  - [ ] Decrease TTL for volatile data (event counts)
  - [ ] Implement cache warming on startup

### PERF-3: Event Processing Latency

#### PERF-3.1: Profiling
**Day 4**

- [ ] **File:** Create `/scripts/profile-api.sh`
  ```bash
  #!/bin/bash
  # Profile API server for 30 seconds

  go tool pprof -http=:8081 http://localhost:8080/debug/pprof/profile?seconds=30
  ```

- [ ] **Analysis:**
  - [ ] Identify CPU hotspots
  - [ ] Identify memory allocations
  - [ ] Identify goroutine leaks

#### PERF-3.2: Optimizations
**Day 5**

- [ ] **Tasks:**
  - [ ] Use sync.Pool for frequently allocated objects
  - [ ] Optimize JSON marshaling (use jsoniter)
  - [ ] Reduce database queries per request
  - [ ] Implement connection pooling adjustments

---

## Success Criteria & Validation

### Phase 11c Validation

- [ ] **Load Test:** 1,000 events/sec sustained for 5 minutes
- [ ] **Latency Test:** p99 publish latency <10ms
- [ ] **Reliability Test:** Zero message loss over 10,000 messages
- [ ] **Failover Test:** Message broker restart recovery <5s
- [ ] **Integration Test:** All 4 providers working (NATS, Kafka, RabbitMQ, Redis)

### Phase 14 Validation

- [ ] **Performance Test:** 30-day report generated in <5s
- [ ] **Export Test:** All formats (PDF, HTML, CSV, JSON) working
- [ ] **Trend Test:** Trend detection accuracy >90%
- [ ] **Schedule Test:** Scheduled report delivered within 1 minute of cron trigger

### Phase 17a Validation

- [ ] **Scale Test:** Manage 10 clusters with 100 agents each
- [ ] **Failover Test:** Cross-cluster failover <30s
- [ ] **Discovery Test:** New agent detected within 1 minute
- [ ] **Aggregation Test:** Cross-cluster event query <500ms

---

## Timeline Summary

### November 27 - December 8 (Phase 11c)
- Week 1: NATS implementation + config integration
- Week 2: Kafka, RabbitMQ, Redis providers
- Week 3: Agent/API integration + testing

### December 9 - December 22 (Phase 14)
- Week 1: Database schema + report engine
- Week 2: Templates + exporters + trend analysis
- Week 3: CLI + scheduler + testing

### December 23 - January 5 (Buffer + Release Prep)
- Holiday period
- v1.1.0 RC testing
- Documentation review
- Bug fixes

### January 6 - January 26 (Phase 17a)
- Week 1-2: Federation API + cluster discovery
- Week 3: Unified alerting + cross-cluster views
- Week 4: Testing + documentation

### Ongoing (Parallel to all phases)
- Quick wins (documentation, config validation)
- Test coverage enhancement
- Performance optimization

---

## Next Steps

1. **Review this roadmap** - Ensure alignment with business priorities
2. **Create GitHub issues** - One issue per major task (MSG-*, RPT-*, MC-*, etc.)
3. **Set up project board** - Track progress across all phases
4. **Start Quick Wins** - Begin documentation and config validation
5. **Phase 11c kickoff** - Design review for messaging framework
6. **Phase 14 parallel start** - Database schema migration can start immediately

---

**For questions or clarifications, see:**
- [CLAUDE.md](CLAUDE.md) - Project overview and architecture
- [docs/](docs/) - Additional documentation
- GitHub Issues - Task-specific discussions
