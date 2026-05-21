# Redis

Langfuse uses Redis (or Valkey) for caching and as a job queue for the Worker component.

::: warning Production guidance
For production workloads use **external** Redis — a managed service (ElastiCache, Memorystore, Aiven, Upstash, Redis Enterprise) or a cluster deployed via a dedicated Redis operator (e.g. spotahome/redis-operator, OT-CONTAINER-KIT/redis-operator). Managed mode in this operator is **single-instance, dev-only** with no Sentinel, no Cluster, and no backups.
:::

## External

Connect to an existing Redis instance:

```yaml
spec:
  redis:
    external:
      secretRef:
        name: langfuse-redis
        keys:
          host: host
          port: port
          password: password     # optional
          tls: tls_enabled       # optional, "true" or "false"
```

## Managed

::: danger Dev / preview only
Managed Redis is a **single-pod StatefulSet** (`replicas: 1`, hardcoded) with AOF persistence on a PVC. No Sentinel, no Cluster, no replication, no backups. On node failure the pod must reschedule and remount its volume before Langfuse can resume. Suitable for local development and CI; **not for production**.

The `replicas` field on `redis.managed` is currently ignored.
:::

Deploy a single-pod Redis for development:

```yaml
spec:
  redis:
    managed:
      storageSize: "10Gi"
```

When using managed mode with no explicit auth secret, the operator auto-generates a Redis password and stores it in the `<instance>-generated-secrets` Secret.

## TLS

For external Redis with TLS enabled, set the `tls` key in your Secret to `"true"`:

```bash
kubectl create secret generic langfuse-redis -n langfuse \
  --from-literal=host="redis.example.com" \
  --from-literal=port="6380" \
  --from-literal=password="secret" \
  --from-literal=tls_enabled="true"
```
