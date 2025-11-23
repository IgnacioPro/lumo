# Roadmap to Vision: From "Tool" to "Platform"

This roadmap breaks down the strategic analysis into granular, actionable engineering tasks. The goal is to transition Lumo from a CLI tool into a monetizable Enterprise SRE Platform.

## Phase 1: The "Action Layer" (Closing the Capability Gap)
**Goal:** Expand the remediation catalog so Lumo can actually *fix* common problems, not just kill processes. This is critical for the "Auto-Remediation" value proposition.

### 1.1 Disk Remediation Actions
- [ ] **Implement `CleanTmpFilesAction`**
  - Logic: Identify files in `/tmp` older than X days.
  - Safety: Verify not currently open (lsof).
  - Risk: Moderate.
- [ ] **Implement `RotateLogsAction`**
  - Logic: Compress/truncate logs in `/var/log` larger than X MB.
  - Safety: Ensure logrotate isn't already running.
  - Risk: Low.
- [ ] **Implement `CheckDiskSpaceAction` (Diagnostic + Minor Fix)**
  - Logic: Clear package manager caches (`apt clean`, `yum clean`).
  - Risk: Safe.

### 1.2 Service Remediation Actions
- [ ] **Implement `RestartSystemdServiceAction`**
  - Logic: `systemctl restart <service>`.
  - Validation: Check `systemctl status` pre/post.
  - Risk: Moderate.
- [ ] **Implement `StopSystemdServiceAction`**
  - Logic: `systemctl stop <service>`.
  - Risk: High (requires approval).
- [ ] **Implement `RestartDockerContainerAction`**
  - Logic: `docker restart <container_id>`.
  - Risk: Moderate.

### 1.3 Kubernetes Remediation Actions (The "Killer Feature")
- [ ] **Implement `K8sRolloutRestartAction`**
  - Logic: `kubectl rollout restart deployment/<name>`.
  - Use case: Stuck pods, config updates.
  - Risk: Moderate.
- [ ] **Implement `K8sScaleDeploymentAction`**
  - Logic: Scale replicas up/down.
  - Use case: Handling traffic spikes.
  - Risk: Moderate.
- [ ] **Implement `K8sDeletePodAction`**
  - Logic: Delete pod to force recreation.
  - Use case: Stuck in `Terminating` or `Error`.
  - Risk: Low/Moderate (if part of ReplicaSet).

---

## Phase 2: The Control Plane (The Monetization Engine)
**Goal:** Build the "Manager" that enterprises pay for. This separates the open-source CLI (single-player) from the commercial platform (multi-player).

### 2.1 API Server Extensions
- [ ] **Agent Management Endpoints**
  - `GET /api/v1/agents`: List all connected agents with status.
  - `GET /api/v1/agents/:id/health`: Detailed health history.
- [ ] **Issue Tracking Endpoints**
  - `GET /api/v1/issues`: Global feed of detected problems.
  - `POST /api/v1/issues/:id/resolve`: Mark as resolved manually.
- [ ] **Approval Workflow Endpoints**
  - `GET /api/v1/approvals`: List pending remediation actions.
  - `POST /api/v1/approvals/:id/approve`: Trigger the action on the agent.
  - `POST /api/v1/approvals/:id/reject`: Cancel the action.

### 2.2 Frontend Dashboard (New Project)
- [ ] **Scaffold `lumo-dashboard`** (Next.js + Tailwind + ShadcnUI).
- [ ] **Dashboard Views:**
  - **Overview:** High-level stats (Healthy vs. Unhealthy hosts).
  - **Agent List:** Filterable table of all infrastructure.
  - **Live Feed:** Real-time stream of incoming diagnostics.
  - **Action Center:** Dedicated UI for approving/rejecting AI recommendations.

---

## Phase 3: Trust & Security (The "Enterprise Grade")
**Goal:** Remove barriers to adoption by ensuring safety and compliance.

### 3.1 Audit Logging (SOC2 Prep)
- [ ] **Implement `AuditService`**
  - Record: Who (User/AI), What (Action), Where (Host), When, Result.
  - Storage: Append-only table in PostgreSQL.
- [ ] **UI View:**
  - Read-only view of the Audit Log in the dashboard.

### 3.2 Input Sanitization & Safety
- [ ] **Command Injection Audit:**
  - Review all `exec.Command` calls.
  - Ensure arguments are never passed via shell string concatenation.
- [ ] **Dry-Run Implementation:**
  - Ensure every `Action` has a `DryRun()` mode that returns *exactly* what command would run without running it.

### 3.3 Authentication & RBAC
- [ ] **Agent Auth:** mTLS or Token-based auth for Agent <-> API communication.
- [ ] **User RBAC:**
  - `Admin`: Can approve Critical actions.
  - `Operator`: Can approve Moderate actions.
  - `Viewer`: Read-only.

---

## Phase 4: Go-to-Market Tech (The "Growth Engine")
**Goal:** Reduce friction for new users.

### 4.1 Installation Experience
- [ ] **One-Line Installer:** `curl | bash` script that detects OS, installs binary, and registers agent.
- [ ] **Docker Compose:** Simple quickstart for self-hosting the API/Dashboard.

### 4.2 Demo Environment
- [ ] **Live Demo:** A read-only instance of the Dashboard connected to dummy agents simulating issues.
