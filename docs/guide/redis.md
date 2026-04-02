# Redis

Langfuse uses Redis (or Valkey) for caching and as a job queue for the Worker component.

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

Deploy a Redis instance managed by the operator:

```yaml
spec:
  redis:
    managed:
      replicas: 3
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
