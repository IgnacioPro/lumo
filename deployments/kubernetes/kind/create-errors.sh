#!/bin/bash

# Configuration
TENANT_NS="tenant-acme-corp"

echo "🔥 Creating chaos in ${TENANT_NS}..."

# Ensure namespace exists
if ! kubectl get ns "${TENANT_NS}" >/dev/null 2>&1; then
    echo "Creating namespace ${TENANT_NS}..."
    kubectl create ns "${TENANT_NS}"
fi

# 1. Create a CrashLoopBackOff Pod
echo "1. Creating CrashLoopBackOff pod..."
cat <<EOF | kubectl apply -n ${TENANT_NS} -f -
apiVersion: v1
kind: Pod
metadata:
  name: simple-crash-loop
  labels:
    test: failure
spec:
  containers:
  - name: crasher
    image: busybox
    command: ["sh", "-c", "exit 1"]
  restartPolicy: Always
EOF

# 2. Create an ImagePullBackOff Pod
echo "2. Creating ImagePullBackOff pod..."
cat <<EOF | kubectl apply -n ${TENANT_NS} -f -
apiVersion: v1
kind: Pod
metadata:
  name: simple-image-pull
  labels:
    test: failure
spec:
  containers:
  - name: puller
    image: invalid-domain.com/missing:latest
  restartPolicy: Always
EOF

echo "✅ Done! Watch the chaos with:"
echo "kubectl get pods -n ${TENANT_NS} -w"
