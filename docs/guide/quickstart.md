# Quick Start

This guide walks through deploying a minimal Langfuse instance with external data stores.

## Prerequisites

Before creating a `LangfuseInstance`, ensure you have:

- The operator [installed](/guide/installation) in your cluster
- PostgreSQL, ClickHouse, and Redis accessible from the cluster
- Connection credentials stored in Kubernetes Secrets

## 1. Create Secrets

Store your data store credentials:

```bash
kubectl create namespace langfuse

# PostgreSQL
kubectl create secret generic langfuse-db -n langfuse \
  --from-literal=database_url="postgresql://user:pass@postgres:5432/langfuse"

# ClickHouse
kubectl create secret generic langfuse-clickhouse -n langfuse \
  --from-literal=url="http://clickhouse:8123" \
  --from-literal=username="default" \
  --from-literal=password="clickhouse-pass"

# Redis
kubectl create secret generic langfuse-redis -n langfuse \
  --from-literal=host="redis" \
  --from-literal=port="6379" \
  --from-literal=password="redis-pass"
```

## 2. Create a LangfuseInstance

```yaml
apiVersion: langfuse.palena.ai/v1alpha1
kind: LangfuseInstance
metadata:
  name: langfuse
  namespace: langfuse
spec:
  image:
    tag: "3"

  auth:
    nextAuthUrl: "https://langfuse.example.com"

  database:
    external:
      secretRef:
        name: langfuse-db
        keys:
          url: database_url

  clickhouse:
    external:
      secretRef:
        name: langfuse-clickhouse
        keys:
          url: url
          username: username
          password: password

  redis:
    external:
      secretRef:
        name: langfuse-redis
        keys:
          host: host
          port: port
          password: password
```

Apply it:

```bash
kubectl apply -f langfuse-instance.yaml
```

## 3. Check Status

```bash
kubectl get langfuseinstance langfuse -n langfuse
```

```
NAME       PHASE     READY   VERSION   AGE
langfuse   Running   true    3         2m
```

Inspect conditions:

```bash
kubectl describe langfuseinstance langfuse -n langfuse
```

## 4. Access Langfuse

The Web component is exposed via a ClusterIP Service on port 3000:

```bash
kubectl port-forward svc/langfuse-web -n langfuse 3000:3000
```

Open `http://localhost:3000` in your browser.

For production access, enable Ingress:

```yaml
spec:
  ingress:
    enabled: true
    className: nginx
    host: langfuse.example.com
    tls:
      enabled: true
      certManager:
        issuerRef:
          name: letsencrypt-prod
          kind: ClusterIssuer
```

## Next Steps

- [Database](/guide/database) &mdash; configure PostgreSQL with CNPG or managed mode
- [Authentication](/guide/authentication) &mdash; set up OIDC or initial admin user
- [Multi-Tenancy](/guide/multi-tenancy) &mdash; manage organizations and projects via CRDs
