# External Secrets Integration for Lumo

This directory contains manifests for integrating external secret managers with Lumo's Kubernetes deployment using the [External Secrets Operator (ESO)](https://external-secrets.io/).

## Overview

**Supported Providers:**
| Provider | Free Tier | Auth Method | Best For |
|----------|-----------|-------------|----------|
| **GCP Secret Manager** | 6 secrets, 10K ops/month | Workload Identity (GKE) or SA key | GCP users, any K8s cluster |
| **AWS Secrets Manager** | 30-day trial, then $0.40/secret/month | IRSA (EKS) or access keys | AWS users |
| **Azure Key Vault** | 10K ops free | Workload Identity (AKS) or SP | Azure users |
| **HashiCorp Vault** | Self-hosted (free) | Token, Kubernetes auth | On-prem, multi-cloud |

> **Note:** GCP Secret Manager works with **any Kubernetes cluster** (GKE, EKS, AKS, on-prem, kind, k3s), not just GKE. You just need a service account key file.

## Why GCP Secret Manager?

- **Free tier:** 6 active secret versions, 10,000 access operations/month
- **Works anywhere:** Any K8s cluster with a service account key
- **Native GKE integration:** Workload Identity (no keys needed)
- **Automatic rotation support**
- **Audit logging via Cloud Audit Logs**

## Prerequisites

1. **GCP Project** with Secret Manager API enabled
2. **Any Kubernetes cluster** (GKE, EKS, AKS, on-prem, kind, etc.)
3. **Authentication:** Workload Identity (GKE only) OR service account key file (any cluster)
4. **External Secrets Operator** installed in cluster

## Quick Start

### 1. Enable GCP Secret Manager API

```bash
gcloud services enable secretmanager.googleapis.com
```

### 2. Create Secrets in GCP

```bash
# Set your project
export GCP_PROJECT_ID="your-project-id"

# Create secrets (run once)
./scripts/create-gcp-secrets.sh

# Or manually:
echo -n "your-jwt-secret" | gcloud secrets create lumo-jwt-secret --data-file=- --project=$GCP_PROJECT_ID
echo -n "your-db-password" | gcloud secrets create lumo-database-password --data-file=- --project=$GCP_PROJECT_ID
echo -n "sk-ant-xxx" | gcloud secrets create lumo-anthropic-api-key --data-file=- --project=$GCP_PROJECT_ID
echo -n "sk-xxx" | gcloud secrets create lumo-openai-api-key --data-file=- --project=$GCP_PROJECT_ID
echo -n "https://hooks.slack.com/..." | gcloud secrets create lumo-slack-webhook-url --data-file=- --project=$GCP_PROJECT_ID
```

### 3. Set Up Service Account

#### Option A: Workload Identity (GKE Only)

Use this if your cluster is running on GKE with Workload Identity enabled.

```bash
# Create GCP service account
gcloud iam service-accounts create lumo-secrets-accessor \
    --display-name="Lumo Secrets Accessor" \
    --project=$GCP_PROJECT_ID

# Grant Secret Manager access
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
    --member="serviceAccount:lumo-secrets-accessor@${GCP_PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"

# Bind to Kubernetes service account (Workload Identity)
gcloud iam service-accounts add-iam-policy-binding \
    lumo-secrets-accessor@${GCP_PROJECT_ID}.iam.gserviceaccount.com \
    --role="roles/iam.workloadIdentityUser" \
    --member="serviceAccount:${GCP_PROJECT_ID}.svc.id.goog[external-secrets/external-secrets]" \
    --project=$GCP_PROJECT_ID
```

#### Option B: Service Account Key (Any Kubernetes Cluster)

Use this for **any cluster**: EKS, AKS, on-prem, kind, k3s, Rancher, OpenShift, etc.

```bash
# Create and download key
gcloud iam service-accounts keys create gcp-sa-key.json \
    --iam-account=lumo-secrets-accessor@${GCP_PROJECT_ID}.iam.gserviceaccount.com

# Create K8s secret with the key
kubectl create secret generic gcp-secret-manager-credentials \
    --from-file=secret-access-credentials=gcp-sa-key.json \
    -n external-secrets

# Delete local key file
rm gcp-sa-key.json
```

### 4. Install External Secrets Operator

```bash
helm repo add external-secrets https://charts.external-secrets.io
helm repo update

helm install external-secrets external-secrets/external-secrets \
    -n external-secrets \
    --create-namespace \
    --set installCRDs=true
```

### 5. Deploy Lumo with GCP Secrets

```bash
# Apply ClusterSecretStore (once per cluster)
kubectl apply -f cluster-secret-store.yaml

# Apply ExternalSecrets (creates K8s secrets from GCP)
kubectl apply -f external-secrets.yaml
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        GCP Secret Manager                        │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ lumo-jwt-secret │  │ lumo-db-password│  │ lumo-ai-keys    │  │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘  │
└───────────┼────────────────────┼────────────────────┼───────────┘
            │                    │                    │
            ▼                    ▼                    ▼
┌─────────────────────────────────────────────────────────────────┐
│              External Secrets Operator (ESO)                     │
│                                                                  │
│  ClusterSecretStore ──► Authenticates with GCP                  │
│  ExternalSecret     ──► Syncs secrets to K8s                    │
└─────────────────────────────────────────────────────────────────┘
            │                    │                    │
            ▼                    ▼                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes Secrets                            │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ lumo-api-secret │  │ lumo-ai-secrets │  │ lumo-agent-secret│ │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘  │
└───────────┼────────────────────┼────────────────────┼───────────┘
            │                    │                    │
            ▼                    ▼                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Lumo Pods                                   │
│  ┌─────────────────┐                    ┌─────────────────┐     │
│  │   lumo-api      │                    │   lumo-agent    │     │
│  └─────────────────┘                    └─────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

## Files

| File | Description |
|------|-------------|
| `cluster-secret-store.yaml` | ClusterSecretStore for GCP Secret Manager authentication |
| `external-secrets.yaml` | ExternalSecret resources that sync GCP secrets to K8s |
| `scripts/create-gcp-secrets.sh` | Script to create secrets in GCP Secret Manager |
| `scripts/setup-workload-identity.sh` | Script to configure Workload Identity |

## Secret Mapping

| GCP Secret Name | K8s Secret | Key | Used By |
|-----------------|------------|-----|---------|
| `lumo-jwt-secret` | `lumo-api-secret` | `jwt-secret` | API Server |
| `lumo-database-password` | `lumo-api-secret` | `database-password` | API Server |
| `lumo-anthropic-api-key` | `lumo-ai-secrets` | `anthropic-api-key` | API Server |
| `lumo-openai-api-key` | `lumo-ai-secrets` | `openai-api-key` | API Server |
| `lumo-agent-token` | `lumo-agent-secret` | `agent-token` | Agent |
| `lumo-slack-webhook-url` | `lumo-notification-secrets` | `slack-webhook-url` | API Server |

## Refresh Interval

By default, secrets are refreshed every 1 hour. This can be configured in `external-secrets.yaml`:

```yaml
spec:
  refreshInterval: 1h  # Options: 1m, 5m, 1h, 24h
```

## Troubleshooting

### Check ESO Status

```bash
# Check operator pods
kubectl get pods -n external-secrets

# Check ExternalSecret sync status
kubectl get externalsecrets -n lumo-system

# Describe for errors
kubectl describe externalsecret lumo-api-secrets -n lumo-system
```

### Common Issues

1. **SecretStore not ready**: Check GCP credentials and IAM permissions
2. **Secret not found**: Verify secret exists in GCP Secret Manager
3. **Permission denied**: Check service account has `secretmanager.secretAccessor` role

### Verify Secrets Created

```bash
kubectl get secrets -n lumo-system
kubectl get secret lumo-api-secret -n lumo-system -o jsonpath='{.data}' | jq
```

## Cost Estimation (Free Tier)

GCP Secret Manager free tier includes:
- **6 active secret versions** (we use ~6)
- **10,000 access operations/month** (plenty for typical deployments)
- **Automatic replication** included

For most Lumo deployments, this will be **completely free**.

## Migration from Hardcoded Secrets

1. Create secrets in GCP Secret Manager
2. Install External Secrets Operator
3. Apply the manifests in this directory
4. Remove hardcoded secrets from deployment manifests
5. Restart pods to pick up new secrets

## Security Best Practices

1. **Rotate secrets regularly** - GCP supports automatic rotation
2. **Use Workload Identity** - Avoid storing service account keys (GKE only)
3. **Enable audit logging** - Track secret access in Cloud Audit Logs
4. **Principle of least privilege** - Only grant `secretAccessor` role
5. **Separate secrets by environment** - Use different GCP projects for prod/staging

---

## Alternative Providers

If you prefer a different secret manager, ESO supports many providers. Here are examples:

### AWS Secrets Manager

```yaml
# ClusterSecretStore for AWS
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: aws-secrets-manager
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-east-1
      # For EKS with IRSA (recommended)
      auth:
        jwt:
          serviceAccountRef:
            name: external-secrets
            namespace: external-secrets
      # Or use access keys (any cluster)
      # auth:
      #   secretRef:
      #     accessKeyIDSecretRef:
      #       name: aws-credentials
      #       key: access-key-id
      #     secretAccessKeySecretRef:
      #       name: aws-credentials
      #       key: secret-access-key
```

### Azure Key Vault

```yaml
# ClusterSecretStore for Azure
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: azure-key-vault
spec:
  provider:
    azurekv:
      vaultUrl: "https://your-vault.vault.azure.net"
      # For AKS with Workload Identity
      authType: WorkloadIdentity
      serviceAccountRef:
        name: external-secrets
        namespace: external-secrets
      # Or use service principal (any cluster)
      # authSecretRef:
      #   clientId:
      #     name: azure-credentials
      #     key: client-id
      #   clientSecret:
      #     name: azure-credentials
      #     key: client-secret
      # tenantId: "your-tenant-id"
```

### HashiCorp Vault (Self-hosted)

Best for on-prem or multi-cloud deployments.

```yaml
# ClusterSecretStore for Vault
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: hashicorp-vault
spec:
  provider:
    vault:
      server: "https://vault.example.com"
      path: "secret"
      version: "v2"
      # Kubernetes auth (recommended)
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "lumo-secrets-reader"
          serviceAccountRef:
            name: external-secrets
            namespace: external-secrets
      # Or use token auth
      # auth:
      #   tokenSecretRef:
      #     name: vault-token
      #     key: token
```

### Switching Providers

1. Create a new `ClusterSecretStore` for your provider
2. Update `external-secrets.yaml` to reference the new store:
   ```yaml
   spec:
     secretStoreRef:
       name: your-new-store  # Change this
       kind: ClusterSecretStore
   ```
3. Create equivalent secrets in your new provider
4. Apply the updated manifests

See [ESO Provider Documentation](https://external-secrets.io/latest/provider/aws-secrets-manager/) for all supported providers.
