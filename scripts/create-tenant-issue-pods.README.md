# Tenant Issue Pod Creator

This script creates problematic pods in tenant namespaces (namespaces that start with "tenant-") to test the Lumo monitoring system's ability to detect and handle various Kubernetes issues in a multi-tenant environment.

## Purpose

The `create-tenant-issue-pods.sh` script is designed to create various types of "issue pods" that exhibit common Kubernetes problems. This allows testing of Lumo's event-driven monitoring capabilities across multiple tenant namespaces (namespaces that start with "tenant-") in a multi-tenant SaaS environment.

## Supported Issue Types

- **crash-loop**: Creates pods that continuously crash and restart
- **oom-kill**: Creates pods that exceed their memory limits and get OOM killed
- **pending**: Creates pods that remain in pending state due to scheduling constraints
- **image-pull-backoff**: Creates pods with invalid/nonexistent images that fail to pull

## Prerequisites

- `kubectl` installed and configured with appropriate permissions
- Access to the target Kubernetes cluster
- Appropriate permissions to create pods and namespaces

## Usage

**Note: Only namespaces starting with "tenant-" are allowed.**

```bash
# Create crash loop pods in a specific tenant namespace
./create-tenant-issue-pods.sh tenant-acme-corp

# Create OOM kill pods in multiple tenant namespaces
./create-tenant-issue-pods.sh -t oom-kill -n 2 tenant-acme-corp tenant-globex

# Create pending pods with custom prefix
./create-tenant-issue-pods.sh --type pending --prefix test-issue tenant-*

# Show help
./create-tenant-issue-pods.sh --help
```

## Options

- `-t, --type`: Issue type to create (default: crash-loop)
- `-n, --number`: Number of issue pods to create per tenant (default: 1)
- `-p, --prefix`: Prefix for pod names (default: issue-pod)
- `-h, --help`: Show help message

## Example Output

After running the script, you can check the created pods:

```bash
# Check pods in a specific tenant namespace
kubectl get pods -n tenant-acme-corp | grep issue-pod

# Check status of created issue pods
kubectl get pods -n tenant-acme-corp
```

## Cleanup

To remove the issue pods after testing:

```bash
# Remove issue pods from a specific tenant
kubectl delete pods -n tenant-acme-corp -l name=issue-pod

# Or delete specific pods
kubectl delete pod issue-pod-tenant-acme-corp-crash-loop-1 -n tenant-acme-corp
```

## Integration with Lumo

These issue pods will be detected by Lumo's event-driven monitoring system:

- Lumo agents will detect the pod issues in real-time
- Events will be generated and sent to the API server
- The system will log and potentially auto-remediate based on configured policies

This enables comprehensive testing of Lumo's multi-tenant monitoring capabilities across different types of Kubernetes failures.