# OpenReplay Kubernetes Installation Guide

## Prerequisites

- Kubernetes cluster v1.18+
- helm 3.10+
- kubectl configured with cluster access
- RWX PVC support for multi-node clusters
- SSL/TLS certificate for HTTPS (Let's Encrypt or custom)

## Installation Steps

### 1. Label the Namespaces

Label the `db` and `app` namespaces with required pod security policies:

```bash
kubectl label namespace db pod-security.kubernetes.io/enforce=privileged
kubectl label namespace db pod-security.kubernetes.io/audit=privileged
kubectl label namespace db pod-security.kubernetes.io/warn=privileged

kubectl label namespace app pod-security.kubernetes.io/enforce=privileged
kubectl label namespace app pod-security.kubernetes.io/audit=privileged
kubectl label namespace app pod-security.kubernetes.io/warn=privileged
```

### 2. Modify Values in vars.yaml

Update the following values in `vars.yaml`:

```yaml
ingress-nginx:
  controller:
    extraArgs:
      default-server-secret: "apps/openreplay.staging.exinity.io-tls"
    config:
      entries:
        ssl-redirect: "true"

global:
  pvcRWXName: "openreplay-shared-storage-pvc"
  domainName: "openreplay.staging.exinity.io"
  
  postgresql:
    postgresqlPassword: "<generate-secure-password>"  # Use: openssl rand -hex 20
    postgresqlHost: "postgresql.db.svc.cluster.local"
    postgresqlPort: "5432"
    postgresqlUser: "postgres"
    postgresqlDatabase: "postgres"
```

### 3. Create Persistent Volume

Create PV/PVC from `pvc.yaml` for shared storage:

```bash
kubectl apply -f pvc.yaml
```

### 4. Setup NFS Storage (for multi-node clusters)

If your cluster has multiple nodes, create NFS with RWX access:

```bash
bash setup-nfs.sh
```

For single-node clusters, the default `hostPath` storage will be used.

### 5. Deploy Database Services

Deploy PostgreSQL, Redis, ClickHouse, and other database services:

```bash
export KUBECONFIG=~/.kube/stg-config  # Adjust path to your kubeconfig
cd openreplay/scripts/helmcharts
helm upgrade --install databases ./databases -n db --create-namespace --wait -f ./vars.yaml --atomic
```

Verify databases are running:
```bash
kubectl get pods -n db
```

### 6. Create SSL/TLS Certificate Secret

If you have a Let's Encrypt or custom SSL certificate in another namespace, copy it to the `apps` namespace first (if needed), then create the OpenReplay SSL secret.

### 7. Create OpenReplay SSL Secret

**Important:** OpenReplay requires HTTPS to function. As per the [OpenReplay Kubernetes documentation](https://docs.openreplay.com/en/deployment/deploy-kubernetes/#bringgenerate-your-ssl-certificate-option-2), you must create a secret named `openreplay-ssl` of type `kubernetes.io/tls`.

#### Option A: Copy from Existing Certificate

If you already have a Let's Encrypt certificate in another namespace:

```bash
# Copy TLS certificate from apps namespace to app namespace
kubectl get secret openreplay.staging.exinity.io-tls -n apps -o json | \
jq '.metadata = {
  "name": "openreplay-ssl",
  "namespace": "app",
  "annotations": {
    "description": "SSL certificate for OpenReplay deployment",
    "source": "copied from apps/openreplay.staging.exinity.io-tls"
  }
} | del(.metadata.ownerReferences, .metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp)' | \
kubectl apply -f -
```

#### Option B: Create from Certificate Files

If you have certificate files (`.crt` and `.key`):

```bash
kubectl create secret tls openreplay-ssl -n app \
  --cert=/path/to/certificate.crt \
  --key=/path/to/private.key
```

#### Option C: Generate with cert-manager

If using cert-manager for automated certificate management:

```bash
cd openreplay/scripts/helmcharts
bash certmanager.sh
# Follow the prompts to generate a Let's Encrypt certificate
```

**Verify the secret:**
```bash
kubectl get secret openreplay-ssl -n app
kubectl get secret openreplay-ssl -n app -o jsonpath='{.data}' | jq 'keys'
# Should show: ["tls.crt", "tls.key"]
```

### 8. Deploy OpenReplay Application

Deploy the OpenReplay application stack:

```bash
export KUBECONFIG=~/.kube/stg-config
cd openreplay/scripts/helmcharts
helm upgrade --install openreplay ./openreplay -n app \
  --create-namespace --wait --timeout 30m -f ./vars.yaml --atomic
```

**Monitor the deployment:**
```bash
# Watch pods starting up
kubectl get pods -n app -w

# Check deployment status
helm list -n app

# View logs of a specific pod if needed
kubectl logs -f <pod-name> -n app
```

### 9. Verify Installation

Once all pods are running:

```bash
# Check all pods are in Running state
kubectl get pods -n app

# Check services
kubectl get svc -n app

# Check ingress
kubectl get ingress -n app
```

Access OpenReplay at: `https://openreplay.staging.exinity.io/signup` (replace with your domain)

### 10. Configure DNS

Point your domain to the LoadBalancer or Ingress IP:

```bash
# Get the external IP
kubectl get ingress -n app openreplay-ingress-nginx-controller

# Create an A record in your DNS provider:
# openreplay.staging.exinity.io -> <EXTERNAL-IP>
```

---

## Troubleshooting

### PostgreSQL Password Authentication Failed

**Problem:** Pods fail to connect to PostgreSQL with error:
```
password authentication failed for user "postgres"
```

**Root Cause:** PostgreSQL was initialized with a different password than what's currently in `vars.yaml`. The database stores password hashes during initialization, and updating Kubernetes secrets doesn't change the actual database password.

**Solution:** Reset the PostgreSQL password without data loss:

1. Backup current pg_hba.conf:
```bash
kubectl exec postgresql-0 -n db -- cat /opt/bitnami/postgresql/conf/pg_hba.conf > /tmp/pg_hba_backup.conf
```

2. Create temporary trust authentication config:
```bash
cat > /tmp/pg_hba_trust.conf << 'EOF'
# PostgreSQL Client Authentication Configuration File - TEMPORARY TRUST MODE
local   all             all                                     trust
host    all             all             127.0.0.1/32            trust
host    all             all             ::1/128                 trust
host    all             all             0.0.0.0/0               md5
EOF
```

3. Apply trust authentication:
```bash
kubectl cp /tmp/pg_hba_trust.conf db/postgresql-0:/tmp/pg_hba_trust.conf
kubectl exec postgresql-0 -n db -- bash -c "cp /tmp/pg_hba_trust.conf /opt/bitnami/postgresql/conf/pg_hba.conf"
kubectl exec postgresql-0 -n db -- bash -c "pg_ctl reload -D /bitnami/postgresql/data"
sleep 3
```

4. Update the PostgreSQL password (replace with your password from vars.yaml):
```bash
kubectl exec postgresql-0 -n db -- psql -U postgres -d postgres -c "ALTER USER postgres WITH PASSWORD 'YOUR_PASSWORD_FROM_VARS_YAML';"
```

5. Restore original pg_hba.conf:
```bash
kubectl cp /tmp/pg_hba_backup.conf db/postgresql-0:/tmp/pg_hba_original.conf
kubectl exec postgresql-0 -n db -- bash -c "cp /tmp/pg_hba_original.conf /opt/bitnami/postgresql/conf/pg_hba.conf"
kubectl exec postgresql-0 -n db -- bash -c "pg_ctl reload -D /bitnami/postgresql/data"
sleep 3
```

6. Verify the password works:
```bash
kubectl exec postgresql-0 -n db -- env PGPASSWORD='YOUR_PASSWORD_FROM_VARS_YAML' psql -U postgres -d postgres -c "SELECT version();"
```

7. Redeploy OpenReplay:
```bash
export KUBECONFIG=~/.kube/stg-config
helm upgrade --install openreplay ./openreplay -n app --wait --timeout 30m -f ./vars.yaml --atomic
```

8. Clean up temporary files:
```bash
rm -f /tmp/pg_hba_backup.conf /tmp/pg_hba_trust.conf
```

**Verification:**
- Check all pods are running: `kubectl get pods -n app`
- Check for authentication errors: `kubectl logs <api-pod-name> -n app | grep "password authentication"`
- Verify helm deployment: `helm list -n app`

---

### MinIO Buckets Not Created

**Problem:** MinIO buckets required for OpenReplay are not automatically created, causing issues with session recordings and assets.

**Required Buckets (9 total):**
- `mobs` - Session recordings
- `sessions-assets` - Session assets (PUBLIC)
- `static` - Static files
- `sourcemaps` - JavaScript sourcemaps
- `sessions-mobile-assets` - Mobile session assets
- `quickwit` - Search engine data
- `vault-data` - Secure vault storage
- `records` - Assist records (Enterprise Edition)
- `spots` - Spot recordings

**Root Cause:** The `databases-migrate` job (responsible for running migrations and creating MinIO buckets) may fail due to:
- PVC mount issues
- Init container failures
- Timeout during migration
- Missing dependencies

**Check if buckets exist:**
```bash
# Get MinIO pod name
kubectl get pods -n db | grep minio

# List buckets (replace <minio-pod-name> with actual pod name)
kubectl exec <minio-pod-name> -n db -- mc alias set local http://localhost:9000 <ACCESS_KEY> <SECRET_KEY>
kubectl exec <minio-pod-name> -n db -- mc ls local/
```

**Solution:** Manually create MinIO buckets if the migration job failed:

1. Get MinIO credentials from `vars.yaml`:
```bash
grep -A 2 "accessKey:" vars.yaml | head -5
# Example output:
# accessKey: &accessKey "8d74a14efbcad5b163de058c303b1fd0d4daac33"
# secretKey: &secretKey "415121ed34651742adcb9ec635bed04a6bb34fb1"
```

2. Create a job to initialize buckets:
```bash
cat << 'EOF' | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: minio-bucket-init
  namespace: app
spec:
  ttlSecondsAfterFinished: 300
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: mc
        image: minio/mc:latest
        command:
        - /bin/sh
        - -c
        - |
          #!/bin/sh
          set -e
          
          # Replace with your actual credentials from vars.yaml
          ACCESS_KEY="YOUR_ACCESS_KEY"
          SECRET_KEY="YOUR_SECRET_KEY"
          
          # Configure mc alias
          mc alias set minio http://minio.db.svc.cluster.local:9000 $ACCESS_KEY $SECRET_KEY
          
          # Create buckets
          buckets="mobs sessions-assets static sourcemaps sessions-mobile-assets quickwit vault-data records spots"
          
          for bucket in $buckets; do
            echo "Creating bucket: $bucket"
            mc mb minio/$bucket || echo "Bucket $bucket already exists"
          done
          
          # Set public policy for sessions-assets
          echo "Setting public policy for sessions-assets"
          mc anonymous set download minio/sessions-assets || true
          
          echo "Bucket initialization complete!"
          mc ls minio/
EOF
```

3. Wait for the job to complete and check logs:
```bash
kubectl wait --for=condition=complete job/minio-bucket-init -n app --timeout=60s
kubectl logs job/minio-bucket-init -n app
```

4. Verify buckets were created:
```bash
kubectl exec <minio-pod-name> -n db -- mc alias set local http://localhost:9000 <ACCESS_KEY> <SECRET_KEY>
kubectl exec <minio-pod-name> -n db -- mc ls local/
```

**Expected Output:**
```
[2026-01-07 11:29:51 UTC]     0B mobs/
[2026-01-07 11:29:51 UTC]     0B quickwit/
[2026-01-07 11:29:51 UTC]     0B records/
[2026-01-07 11:29:51 UTC]     0B sessions-assets/
[2026-01-07 11:29:51 UTC]     0B sessions-mobile-assets/
[2026-01-07 11:29:51 UTC]     0B sourcemaps/
[2026-01-07 11:29:51 UTC]     0B spots/
[2026-01-07 11:29:51 UTC]     0B static/
[2026-01-07 11:29:51 UTC]     0B vault-data/
```

**Cleanup:**
```bash
# Optional: Remove the bucket initialization job after success
kubectl delete job minio-bucket-init -n app
```

**Prevention for Future Deployments:**
- Ensure PVC `openreplay-shared-storage-pvc` is accessible before deployment
- Verify database services are ready before the migration job runs
- Check migration job status: `kubectl get jobs -n app`
- Check job logs if it fails: `kubectl logs job/databases-migrate -n app`

---

### DNS Not Pointing to Correct IP

**Problem:** OpenReplay is not accessible at the configured domain.

**Symptoms:**
- Cannot access `https://openreplay.staging.exinity.io`
- Browser shows connection timeout or wrong page

**Diagnosis:**
```bash
# Check OpenReplay ingress LoadBalancer IP
kubectl get svc openreplay-ingress-nginx-controller -n app
# Note the EXTERNAL-IP (e.g., 10.20.30.152)

# Check DNS resolution
nslookup openreplay.staging.exinity.io
# or
dig +short openreplay.staging.exinity.io
```

**Root Cause:** DNS A record points to wrong IP address (often an old/different ingress controller).

**Solution:**
1. Get the correct LoadBalancer IP:
```bash
kubectl get svc openreplay-ingress-nginx-controller -n app -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

2. Update DNS A record at your DNS provider:
   - Domain: `openreplay.staging.exinity.io`
   - Type: A
   - Value: `<EXTERNAL-IP from step 1>`
   - TTL: 300 (or as per your policy)

3. Wait for DNS propagation (5-60 minutes depending on TTL)

4. Verify DNS update:
```bash
nslookup openreplay.staging.exinity.io
# Should now show the correct IP
```

**Temporary Testing (bypass DNS):**

Add to `/etc/hosts` for immediate testing:
```bash
sudo sh -c 'echo "<EXTERNAL-IP> openreplay.staging.exinity.io" >> /etc/hosts'
```

Then test: `https://openreplay.staging.exinity.io/signup`

**Remember to remove the hosts entry after DNS is fixed:**
```bash
sudo sed -i.bak '/openreplay.staging.exinity.io/d' /etc/hosts
```

**Verification:**
- DNS resolves to correct IP: `nslookup openreplay.staging.exinity.io`
- Can access OpenReplay: `curl -I https://openreplay.staging.exinity.io`
- Signup page loads: Visit `https://openreplay.staging.exinity.io/signup` in browser
