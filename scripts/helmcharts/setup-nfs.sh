#!/bin/bash
set -e

echo "=== Setting up NFS for OpenReplay ==="

# Step 1: Delete the pending PVC
echo "Cleaning up pending PVC..."
kubectl delete pvc openreplay-shared-storage-pvc -n app 2>/dev/null || true

# Step 2: Create NFS server namespace
echo "Creating NFS server namespace..."
kubectl create namespace nfs-server 2>/dev/null || true

# Step 3: Create NFS server with RWO storage
echo "Creating NFS server..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nfs-server-pvc
  namespace: nfs-server
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Gi
  storageClassName: k8s-rabbit-staging-cluster
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nfs-server
  namespace: nfs-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nfs-server
  template:
    metadata:
      labels:
        app: nfs-server
    spec:
      containers:
      - name: nfs-server
        image: itsthenetwork/nfs-server-alpine:latest
        ports:
        - name: nfs
          containerPort: 2049
        - name: mountd
          containerPort: 20048
        - name: rpcbind
          containerPort: 111
        securityContext:
          privileged: true
        volumeMounts:
        - name: nfs-storage
          mountPath: /nfsshare
        env:
        - name: SHARED_DIRECTORY
          value: /nfsshare
      volumes:
      - name: nfs-storage
        persistentVolumeClaim:
          claimName: nfs-server-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: nfs-server
  namespace: nfs-server
spec:
  ports:
  - name: nfs
    port: 2049
  - name: mountd
    port: 20048
  - name: rpcbind
    port: 111
  selector:
    app: nfs-server
  clusterIP: None
EOF

# Step 4: Wait for NFS server to be ready
echo "Waiting for NFS server to be ready..."
kubectl wait --for=condition=ready pod -l app=nfs-server -n nfs-server --timeout=5m

# Step 5: Get NFS server IP
NFS_SERVER_IP=$(kubectl get pod -n nfs-server -l app=nfs-server -o jsonpath='{.items[0].status.podIP}')
echo "NFS Server IP: $NFS_SERVER_IP"

# Step 6: Add NFS provisioner repo
echo "Adding NFS provisioner Helm repo..."
helm repo add nfs-subdir-external-provisioner https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner/ 2>/dev/null || true
helm repo update

# Step 7: Install NFS provisioner
echo "Installing NFS provisioner..."
helm upgrade --install nfs-provisioner nfs-subdir-external-provisioner/nfs-subdir-external-provisioner \
  --namespace nfs-provisioner --create-namespace \
  --set nfs.server=$NFS_SERVER_IP \
  --set nfs.path=/ \
  --set storageClass.name=nfs-rwx \
  --set storageClass.defaultClass=false \
  --wait

# Step 8: Create OpenReplay PVC with NFS
echo "Creating OpenReplay PVC with NFS..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: openreplay-shared-storage-pvc
  namespace: app
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 60Gi
  storageClassName: nfs-rwx
EOF

# Step 9: Wait for PVC to be bound
echo "Waiting for PVC to be bound..."
kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc/openreplay-shared-storage-pvc -n app --timeout=2m

echo "=== NFS setup complete! ==="
kubectl get pvc openreplay-shared-storage-pvc -n app