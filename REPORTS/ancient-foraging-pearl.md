# Smart Notification Defaults System - Implementation Plan

## Overview

Implement an intelligent notification system that reduces alert fatigue by ~80% while maintaining zero false negatives for critical events. Uses a **5-layer defense system** with AI-driven decision making.

**Target:** 1-2 weeks implementation, config-first approach, minimal code changes (~800 LOC)

---

## Architecture: 5-Layer Defense System

```
K8s Event → Watcher → [1. Adaptive Debouncer] → API Server
                              (agent-side)
                                                      ↓
                                        [2. AI Actionability Scoring]
                                            (1-10 score filter)
                                                      ↓
                                              [3. Quiet Hours Filter]
                                            (time-based suppression)
                                                      ↓
                                                [4. Event Batcher]
                                              (5-min group window)
                                                      ↓
                                        [5. Per-Provider Rate Limiter]
                                              (token bucket 10/hr)
                                                      ↓
                                              Notification Sent
```

**Expected Impact:**
- Notification volume: 80% reduction (50/hour → 10/hour)
- False positive rate: <5% (down from ~30%)
- Critical event latency: <60 seconds (no degradation)
- Zero false negatives for critical events

---

## Configuration Schema (Sensible Defaults)

### Location: `/configs/config.example.yaml` + `/internal/config/config.go`

```yaml
notifications:
  enabled: true
  notifiers: [...]  # Existing config unchanged

  # NEW: Smart Alerting Configuration
  smart_alerting:
    enabled: true  # Master kill switch for emergency rollback

    # Layer 2: AI Actionability Scoring
    ai_scoring:
      enabled: true
      min_score_threshold: 5  # Suppress events scored < 5 (1-10 scale)
      # AI scores: 1-2=info, 3-4=can wait, 5-6=notify, 7-8=important, 9-10=critical

    # Layer 4: Event Batching
    batching:
      enabled: true
      window: 5m               # Wait 5 minutes to collect related events
      max_batch_size: 10       # Process immediately if 10+ events
      group_by: owner_uid      # Group pod failures by Deployment/StatefulSet

    # Layer 5: Per-Channel Rate Limiting
    rate_limiting:
      enabled: true
      default_max_per_hour: 10
      critical_bypass: true    # Critical events ignore rate limits
      overrides:
        - provider: slack
          max_per_hour: 20     # Higher limit for Slack
        - provider: email
          max_per_hour: 5      # Lower limit for email
      burst_size: 3            # Allow 3 immediate notifications

    # Layer 3: Quiet Hours
    quiet_hours:
      enabled: true
      timezone: America/Los_Angeles
      start_time: "22:00"      # 10 PM
      end_time: "08:00"        # 8 AM
      suppress_severities: [low, medium]  # Critical/high always notify

# Layer 1: Adaptive Debouncing (agent-side)
agent:
  event_driven:
    adaptive_debouncing:
      enabled: true
      windows:
        critical: 15s   # Faster alerts for critical issues
        high: 45s       # Current default
        medium: 90s     # Wait longer for medium severity
        low: 180s       # Much longer for informational events
```

---

## Implementation Phases (12 Days)

### Phase 1: Config Schema + Adaptive Debouncing (2-3 days)

**Goal:** Enable severity-based debounce windows (agent-side filtering)

**Files to Modify:**

1. **`internal/config/config.go`** (~80 lines)
   - Add `SmartAlertingConfig` struct with 5 nested config structs
   - Add `AdaptiveDebouncingConfig` with `map[Severity]time.Duration`
   - Add Viper bindings for environment variable overrides

2. **`internal/agent/eventdriven/debouncer.go`** (~30 lines)
   - Modify `getDebounceWindow()` to select window by severity
   - Update `DebouncerConfig` to accept adaptive windows map

3. **`configs/config.example.yaml`** (~60 lines)
   - Document all new options with inline comments

**Testing:**
- Config loading with adaptive windows
- Verify critical events use 15s, low events use 180s
- Environment variable overrides work

**Expected Outcome:** Critical events notify faster (15s vs 45s), low-severity events wait longer (reduces noise)

---

### Phase 2: AI Actionability Scoring (2-3 days)

**Goal:** AI scores events 1-10, suppress low-score alerts

**Files to Modify:**

1. **`internal/api/handlers/events.go`** (~120 lines)
   - Enhance `analyzeEventWithAI()` to request actionability score
   - Parse AI response to extract score (structured format)
   - Add `shouldNotifyBasedOnScore()` filter logic
   - Store score in database for analytics

2. **`internal/database/models/event.go`** (~10 lines)
   - Add `ActionabilityScore int` field

3. **Migration:** `internal/database/migrations/` (~20 lines)
   ```sql
   ALTER TABLE events ADD COLUMN actionability_score INTEGER;
   CREATE INDEX idx_events_actionability_score ON events(actionability_score);
   ```

**AI Prompt Design:**

```
A Kubernetes event has been detected:

Event Type: {{event_type}}
Severity: {{severity}}
Resource: {{resource_kind}}/{{resource_name}}
Namespace: {{namespace}}
Message: {{message}}
Count: {{count}} occurrences in last hour
First Seen: {{first_seen}}

Analyze this event and provide:
1. Root cause analysis
2. Impact assessment
3. Recommended remediation steps
4. **ACTIONABILITY SCORE (1-10)**

Scoring Guidelines:
- 1-2: Informational, no action needed (e.g., normal pod restarts during deployment)
- 3-4: Can wait, monitor for patterns (e.g., single pod pending <2 minutes)
- 5-6: Should notify team (e.g., image pull failures, resource constraints)
- 7-8: Needs attention soon (e.g., repeated failures, degraded service)
- 9-10: Immediate action required (e.g., production outage, all replicas down)

Consider:
- Event frequency (count={{count}})
- Duration (first_seen={{first_seen}})
- Resource criticality (namespace={{namespace}})
- Likelihood of self-resolution

Response Format:
ACTIONABILITY_SCORE: <number>
ROOT_CAUSE: <analysis>
IMPACT: <assessment>
RECOMMENDED_ACTION: <steps>
```

**Scoring Logic:**
```go
func (h *EventsHandler) shouldNotifyBasedOnScore(score int, severity string) bool {
    // Critical events always notify (bypass AI scoring)
    if severity == "critical" {
        return true
    }

    // Use configured threshold (default: 5)
    return score >= h.config.SmartAlerting.AIScoring.MinScoreThreshold
}
```

**Testing:**
- AI response parsing (handle various formats)
- Score threshold filtering (score 4 → suppressed, score 6 → notified)
- Critical bypass (always notify regardless of score)
- AI failure handling (default to medium score, notify)

**Expected Outcome:** ~40% of high-severity events suppressed as transient/informational

---

### Phase 3: Event Batching (2-3 days)

**Goal:** Group related events (same Deployment/StatefulSet) into single notification

**Files to Modify:**

1. **`internal/api/handlers/events.go`** (~100 lines)
   - Add `EventBatcher` struct with Redis-backed state
   - Implement `AddEvent()` with time-window grouping
   - Add `processBatch()` to send grouped notification
   - Create `buildBatchNotificationMessage()` for multi-event formatting

**EventBatcher Design:**
```go
type EventBatcher struct {
    mu           sync.RWMutex
    batches      map[string]*EventBatch  // key = OwnerUID
    window       time.Duration           // 5 minutes default
    maxBatchSize int                     // 10 events default
    redis        *redis.Client
}

type EventBatch struct {
    Events      []*models.Event
    FirstSeen   time.Time
    Timer       *time.Timer
}

func (b *EventBatcher) AddEvent(event *models.Event) {
    key := event.OwnerUID  // Group by Deployment/StatefulSet

    batch := b.getOrCreateBatch(key)
    batch.Events = append(batch.Events, event)

    // Process immediately if batch full
    if len(batch.Events) >= b.maxBatchSize {
        b.processBatch(key, batch)
        return
    }

    // Otherwise wait for window expiry
    if batch.Timer == nil {
        batch.Timer = time.AfterFunc(b.window, func() {
            b.processBatch(key, batch)
        })
    }
}
```

**Batch Notification Format:**
```
🔔 5 Kubernetes Events Detected - Deployment/nginx (production)

Summary:
- 🔴 Critical: 3 (OOMKilled × 3)
- 🟠 High: 2 (CrashLoopBackOff × 2)

Events:
1. OOMKilled - pod/nginx-7d8f4c9b-abc12
2. OOMKilled - pod/nginx-7d8f4c9b-def34
3. OOMKilled - pod/nginx-7d8f4c9b-ghi56
4. CrashLoopBackOff - pod/nginx-7d8f4c9b-jkl78
5. CrashLoopBackOff - pod/nginx-7d8f4c9b-mno90

AI Analysis (Composite):
All pods in nginx deployment experiencing memory limits (128Mi insufficient).
Average memory usage: 142Mi (+11% over limit).

Recommended Actions:
1. Increase memory limit to 256Mi
2. Add resource requests for proper scheduling
3. Review for potential memory leaks

Impact: Service degraded, 0/5 replicas available
```

**Testing:**
- Time window expiration (5 minutes)
- Max batch size trigger (10 events)
- Grouping by OwnerUID (same Deployment)
- Notification formatting (summary + details)

**Expected Outcome:** ~50% notification reduction (5 events → 1 notification)

---

### Phase 4: Rate Limiting + Quiet Hours (2-3 days)

**Goal:** Per-provider rate limits and time-based suppression

**Files to Modify:**

1. **`internal/api/handlers/events.go`** (~100 lines)
   - Add `NotificationRateLimiter` with Redis token bucket
   - Add `QuietHoursFilter` with timezone handling
   - Integrate both filters into notification pipeline

**Rate Limiter Design:**
```go
type NotificationRateLimiter struct {
    cache             *redis.Client
    defaultMaxPerHour int
    providerLimits    map[string]int
    criticalBypass    bool
    burstSize         int
}

func (r *NotificationRateLimiter) AllowNotification(ctx, provider, severity) (bool, error) {
    // Critical events bypass rate limits
    if r.criticalBypass && severity == "critical" {
        return true, nil
    }

    // Redis key: lumo:ratelimit:notifications:<provider>:<hour>
    hour := time.Now().Truncate(time.Hour).Unix()
    key := fmt.Sprintf("lumo:ratelimit:notifications:%s:%d", provider, hour)

    count, _ := r.cache.Incr(ctx, key).Result()
    if count == 1 {
        r.cache.Expire(ctx, key, 2*time.Hour)  // Auto-cleanup
    }

    limit := r.getLimit(provider)

    // Allow burst (first N notifications always allowed)
    if count <= r.burstSize {
        return true, nil
    }

    return count <= limit, nil
}
```

**Quiet Hours Filter:**
```go
type QuietHoursFilter struct {
    timezone           *time.Location
    startTime          time.Time  // HH:MM (22:00)
    endTime            time.Time  // HH:MM (08:00)
    suppressSeverities map[string]bool
}

func (f *QuietHoursFilter) ShouldSuppress(severity string) bool {
    // Always allow critical/high severity
    if !f.suppressSeverities[severity] {
        return false
    }

    now := time.Now().In(f.timezone)
    currentTime := time.Date(0, 1, 1, now.Hour(), now.Minute(), 0, 0, time.UTC)

    // Handle overnight periods (22:00-08:00)
    if f.startTime.After(f.endTime) {
        return currentTime.After(f.startTime) || currentTime.Before(f.endTime)
    }

    return currentTime.After(f.startTime) && currentTime.Before(f.endTime)
}
```

**Testing:**
- Token bucket (10/hour limit)
- Burst allowance (first 3 always allowed)
- Critical bypass (always notify)
- Timezone conversion (America/Los_Angeles)
- Overnight periods (22:00-08:00 spans midnight)

**Expected Outcome:** No notification spam during off-hours, rate limit protection

---

### Phase 5: Integration + Testing (2-3 days)

**Goal:** Wire all layers together, comprehensive testing

**Integration Points:**

1. **`internal/api/handlers/events.go`** - Main orchestration
   ```go
   func (h *EventsHandler) processSingleEvent(ctx, event) error {
       // Layer 2: AI Actionability Scoring
       aiResp, err := h.analyzeEventWithAI(ctx, event)
       if err != nil {
           log.WithError(err).Warn("AI analysis failed, defaulting to notify")
       } else if !h.shouldNotifyBasedOnScore(aiResp.Score, event.Severity) {
           log.WithField("score", aiResp.Score).Info("Event suppressed by AI scoring")
           return nil
       }

       // Layer 3: Quiet Hours
       if h.quietHoursFilter.ShouldSuppress(event.Severity) {
           log.Info("Event suppressed by quiet hours")
           return nil
       }

       // Layer 4: Event Batching
       h.eventBatcher.AddEvent(event)

       return nil
   }

   func (h *EventsHandler) sendNotifications(ctx, events []*models.Event) error {
       for _, provider := range h.notificationProviders {
           // Layer 5: Rate Limiting
           allowed, err := h.rateLimiter.AllowNotification(ctx, provider.Name, event.Severity)
           if !allowed {
               log.WithField("provider", provider.Name).Warn("Notification rate limited")
               continue
           }

           // Send notification
           notification := h.buildNotification(events)
           provider.Send(ctx, notification)
       }
   }
   ```

2. **Metrics & Observability**
   ```go
   // Add Prometheus metrics
   eventsSuppressedTotal := prometheus.NewCounterVec(
       prometheus.CounterOpts{Name: "lumo_events_suppressed_total"},
       []string{"reason"},  // ai_score, quiet_hours, rate_limit
   )

   eventsBatchedTotal := prometheus.NewCounter(
       prometheus.CounterOpts{Name: "lumo_events_batched_total"},
   )
   ```

**Testing Strategy:**

1. **Unit Tests** (~200 LOC)
   - AI score parsing
   - Quiet hours timezone handling
   - Rate limiter token bucket
   - Event batching window logic

2. **Integration Tests** (~150 LOC)
   - End-to-end event flow with all layers
   - Critical bypass verification (must never suppress)
   - Batch notification formatting
   - Rate limit enforcement

3. **Chaos Testing**
   - AI provider failure → defaults to notify
   - Redis failure → fail-open (allow notifications)
   - Concurrent event storm (100+ events)

**Success Criteria:**
- All tests pass
- Zero false negatives for critical events
- Notification volume reduction visible in metrics
- No performance degradation (<100ms added latency)

---

## Critical Files for Implementation

### Must Modify (Core)
1. **`internal/config/config.go`** - All config structs (~200 lines)
2. **`internal/agent/eventdriven/debouncer.go`** - Adaptive windows (~30 lines)
3. **`internal/api/handlers/events.go`** - All filtering layers (~400 lines)

### Database
4. **`internal/database/models/event.go`** - Add actionability_score field
5. **`internal/database/repository/event.go`** - Score update methods
6. **`internal/database/migrations/`** - New migration file

### Configuration
7. **`configs/config.example.yaml`** - Documentation (~100 lines)
8. **`deployments/kubernetes/base/configmap-agent.yaml`** - K8s config

### Testing
9. **`tests/integration/events_filtering_test.go`** - New test file
10. **`internal/api/handlers/events_test.go`** - Enhanced existing tests

---

## Migration & Rollout Strategy

### Week-by-Week Rollout (5 weeks)

**Week 1: Adaptive Debouncing**
```yaml
agent:
  event_driven:
    adaptive_debouncing:
      enabled: true
```
- Monitor: Critical event latency (should improve to 15s)
- Validate: Low-severity events wait longer (180s)

**Week 2: AI Scoring (Conservative)**
```yaml
notifications:
  smart_alerting:
    ai_scoring:
      enabled: true
      min_score_threshold: 3  # Very permissive, increase later
```
- Monitor: AI scoring accuracy vs manual review
- Adjust: Increase threshold to 5 after validation

**Week 3: Event Batching**
```yaml
notifications:
  smart_alerting:
    batching:
      enabled: true
```
- Monitor: Notification volume reduction
- Validate: Related events grouped correctly

**Week 4: Rate Limiting (High Limit)**
```yaml
notifications:
  smart_alerting:
    rate_limiting:
      enabled: true
      default_max_per_hour: 20  # Double initial limit
```
- Monitor: Rate limit hits
- Adjust: Decrease to 10 after stable

**Week 5: Quiet Hours**
```yaml
notifications:
  smart_alerting:
    quiet_hours:
      enabled: true
      suppress_severities: [low]  # Only low initially
```
- Monitor: After-hours notification volume
- Adjust: Add medium severity if safe

### Emergency Rollback

Single flag disables entire system:
```yaml
notifications:
  smart_alerting:
    enabled: false  # Reverts to current behavior
```

---

## Risk Mitigation & Safeguards

### Critical Safeguards

1. **Critical Events Always Notify**
   - Bypass ALL filters (AI, rate limiting, quiet hours)
   - OOMKilled, NodeNotReady, JobFailed always alert

2. **AI Failure = Fail-Safe**
   - If AI analysis fails → default to score 6 (notify)
   - Never suppress due to AI errors

3. **Redis Failure = Fail-Open**
   - If Redis unavailable → allow all notifications
   - Graceful degradation, no silent failures

4. **Monitoring & Alerts**
   - Prometheus metric: `lumo_events_suppressed_critical_total` (should always be 0)
   - Alert if critical event suppressed
   - Dashboard showing suppression reasons

### Validation Queries

```sql
-- Verify no critical events suppressed
SELECT COUNT(*)
FROM events
WHERE severity = 'critical'
  AND notification_sent = false
  AND created_at > NOW() - INTERVAL '7 days';
-- Expected: 0

-- Suppression breakdown
SELECT
    suppression_reason,
    COUNT(*) as count,
    AVG(actionability_score) as avg_score
FROM events
WHERE notification_sent = false
GROUP BY suppression_reason;

-- Notification volume trend
SELECT
    DATE_TRUNC('day', created_at) as day,
    COUNT(*) as total_events,
    SUM(CASE WHEN notification_sent THEN 1 ELSE 0 END) as notified,
    ROUND(100.0 * SUM(CASE WHEN notification_sent THEN 1 ELSE 0 END) / COUNT(*), 2) as notify_pct
FROM events
GROUP BY day
ORDER BY day DESC
LIMIT 30;
```

---

## Success Metrics (30-Day Measurement)

### Quantitative Goals

| Metric | Before | After | Target Improvement |
|--------|--------|-------|-------------------|
| Notifications/hour | 50 | 10 | 80% reduction |
| False positive rate | ~30% | <5% | 83% reduction |
| Alert ack time | ~15 min | <5 min | 67% improvement |
| Critical event latency | 45s avg | <30s avg | 33% improvement |
| False negatives (critical) | 0 | 0 | Zero tolerance |

### Qualitative Goals

- Engineers report reduced alert fatigue
- On-call rotation less stressful
- Faster incident response (actionable alerts)
- Higher confidence in alert accuracy

---

## Summary

This plan implements a comprehensive smart notification system using **5 defensive layers**:

1. **Adaptive Debouncing** (agent): Faster critical alerts, longer low-severity waits
2. **AI Actionability Scoring**: AI decides if human intervention needed (1-10 scale)
3. **Quiet Hours**: Time-based suppression for low/medium severity
4. **Event Batching**: Group related failures (5-min window, OwnerUID grouping)
5. **Rate Limiting**: Token bucket per provider (10/hour default)

**Key Principles:**
- Config-first approach (minimal code)
- AI-driven decisions (trust AI scoring)
- Fail-safe design (critical events always notify)
- Gradual rollout (5-week phased deployment)
- Zero risk to critical alerts

**Timeline:** 12 working days (~2.5 weeks)
**Code Changes:** ~800 LOC total
**Expected Impact:** 80% notification reduction, <5% false positive rate
