# Event Types

This document describes all Kubernetes event types that Lumo detects and processes.

## Severity Levels

Events are classified by severity to help prioritize responses:

| Severity | Description | Response Time |
|----------|-------------|---------------|
| **critical** | Immediate action required, service impacting | < 5 minutes |
| **high** | Urgent attention needed, potential impact | < 15 minutes |
| **medium** | Should be addressed soon | < 1 hour |
| **low** | Informational, schedule for review | Next business day |

## Event Types

### Critical Severity

#### `oom-killed`
**Resource:** Pod  
**Trigger:** Container terminated due to Out-Of-Memory condition  
**Detection:** `State.Terminated.Reason == "OOMKilled"` or `LastTerminationState.Terminated.Reason == "OOMKilled"`

**Common Causes:**
- Memory limits set too low
- Memory leak in application
- Unexpected workload spike

**Remediation:**
- Increase memory limits
- Profile application memory usage
- Check for memory leaks

---

#### `pod-evicted`
**Resource:** Pod  
**Trigger:** Pod forcefully removed from node  
**Detection:** `Status.Phase == "Failed"` with eviction reason

**Common Causes:**
- Node resource pressure (memory, disk)
- Node maintenance
- Priority-based preemption

**Remediation:**
- Review node resources
- Check PriorityClass settings
- Consider pod disruption budgets

---

#### `node-not-ready`
**Resource:** Node  
**Trigger:** Node condition changes to NotReady  
**Detection:** `NodeReady` condition status changes to `False` or `Unknown`

**Common Causes:**
- Node network issues
- kubelet crash or hang
- System resource exhaustion

**Remediation:**
- Check node connectivity
- Review kubelet logs
- Inspect system resources

---

#### `job-failed`
**Resource:** Job  
**Trigger:** Job exceeds backoff limit  
**Detection:** `Status.Failed > 0` with `BackoffLimitExceeded` reason

**Common Causes:**
- Application error in job
- Resource constraints
- External dependency failure

**Remediation:**
- Review job logs
- Check resource limits
- Verify external dependencies

---

### High Severity

#### `image-pull-backoff`
**Resource:** Pod  
**Trigger:** Repeated failures pulling container image  
**Detection:** Container waiting with `ImagePullBackOff` or `ErrImagePull` reason

**Common Causes:**
- Invalid image name or tag
- Registry authentication issues
- Network connectivity to registry

**Remediation:**
- Verify image name and tag
- Check imagePullSecrets
- Test registry connectivity

---

#### `crash-loop-backoff`
**Resource:** Pod  
**Trigger:** Container repeatedly crashing  
**Detection:** Container waiting with `CrashLoopBackOff` reason and `RestartCount > 0`

**Common Causes:**
- Application startup failure
- Missing dependencies
- Configuration errors
- Liveness probe failures

**Remediation:**
- Check container logs
- Review startup/liveness probes
- Verify environment configuration

---

#### `deployment-failed`
**Resource:** Deployment  
**Trigger:** Deployment progress stalled  
**Detection:** `Progressing` condition with `ProgressDeadlineExceeded` reason

**Common Causes:**
- Insufficient cluster resources
- Image pull failures
- Pod scheduling issues

**Remediation:**
- Check pod status and events
- Review resource quotas
- Verify node capacity

---

#### `volume-failed-mount`
**Resource:** Pod  
**Trigger:** Persistent volume mount failure  
**Detection:** Warning events with `FailedMount` or `FailedAttachVolume` reason

**Common Causes:**
- Volume not available
- Incorrect mount options
- Permission issues

**Remediation:**
- Check PV/PVC status
- Verify storage class configuration
- Review mount options

---

### Medium Severity

#### `pvc-provision-failed`
**Resource:** PersistentVolumeClaim  
**Trigger:** PVC provisioning timeout (>2 minutes)  
**Detection:** PVC in `Pending` state with `ProvisioningFailed` event or age > 2 minutes

**Common Causes:**
- Storage class misconfiguration
- Storage backend issues
- Resource quota exceeded

**Remediation:**
- Check storage class
- Verify storage backend health
- Review resource quotas

---

#### `pod-pending`
**Resource:** Pod  
**Trigger:** Pod stuck in Pending state (>5 minutes)  
**Detection:** `Status.Phase == "Pending"` for extended duration

**Common Causes:**
- Insufficient node resources
- Node selector/affinity constraints
- PVC pending

**Remediation:**
- Check pod events
- Review node resources
- Verify scheduling constraints

---

#### `high-restart-count`
**Resource:** Pod  
**Trigger:** Container restart count exceeds threshold (default: 5)  
**Detection:** `RestartCount > threshold`

**Common Causes:**
- Intermittent failures
- Resource exhaustion
- External dependency issues

**Remediation:**
- Review container logs
- Check resource limits
- Investigate failure pattern

---

### Low Severity

#### `pod-created`
**Resource:** Pod  
**Trigger:** New pod created  
**Detection:** Pod add event via informer

**Notes:**
- Informational event
- Useful for audit trails

---

#### `pod-deleted`
**Resource:** Pod  
**Trigger:** Pod deleted  
**Detection:** Pod delete event via informer

**Notes:**
- Informational event
- Normal during rollouts

---

## Filtering Events

Use the `lumo events` command to query events:

```bash
# Filter by severity
lumo events --severity critical
lumo events --severity high,critical

# Filter by type
lumo events --type oom-killed
lumo events --type crash-loop-backoff

# Filter by namespace
lumo events --namespace production

# Time-based filtering
lumo events --since 24h
lumo events --since 7d

# Combine filters
lumo events --severity critical --namespace production --since 1h
```

## API Endpoints

Events can also be queried via the REST API:

```bash
# List events
GET /api/v1/events

# Query parameters
GET /api/v1/events?severity=critical&limit=50
GET /api/v1/events?type=oom-killed&since=24h
GET /api/v1/events?namespace=production
```

See the [API documentation](getting-started.md) for full details.
