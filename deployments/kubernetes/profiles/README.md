# Lumo Deployment Profiles

Choose the right messaging profile based on your scale and requirements.

## Quick Selection

| Agents | Profile | Provider | Throughput | RAM | Setup Time |
|--------|---------|----------|------------|-----|------------|
| 0-10 | **Startup** | Redis Streams | ~100/sec | 0MB | 0 min |
| 10-50 | **Small Business** | NATS | 1,000+/sec | 64MB | 5 min |
| 50-500 | **Enterprise** | NATS Cluster | 10,000+/sec | 1.5GB | 15 min |
| 500+ | **Hyperscale** | Kafka | 100,000+/sec | 10GB | 30 min |

## Profile Usage in Code

```go
import "github.com/ignacio/lumo/internal/messaging"

// Automatic recommendation based on agent count
profile := messaging.ProfileRecommendation(agentCount)
config, _ := messaging.ProfileConfig(profile)

// Or explicitly choose
config, _ := messaging.ProfileConfig(messaging.ProfileSmallBusiness)
```

## Profile Details

### 🚀 Startup (0-10 agents)
- **Provider:** Redis Streams  
- **Cost:** $0 (uses existing Redis)
- **Setup:** Instant
- **Best for:** POC, dev, small teams

### 💼 Small Business (10-50 agents)  
- **Provider:** NATS single node
- **Cost:** ~$7/month
- **Setup:** 5 minutes
- **Best for:** Production SMB, 1 datacenter

### 🏢 Enterprise (50-500 agents)
- **Provider:** NATS cluster (3 nodes)
- **Cost:** ~$60/month  
- **Setup:** 15 minutes
- **Best for:** HA, multi-AZ, enterprise SLA

### 🌐 Hyperscale (500+ agents)
- **Provider:** Kafka cluster
- **Cost:** ~$500/month
- **Setup:** 30 minutes  
- **Best for:** Global scale, multi-cluster, replay

## See Full Documentation

For detailed deployment instructions, see main messaging README:
`/internal/messaging/README.md`
