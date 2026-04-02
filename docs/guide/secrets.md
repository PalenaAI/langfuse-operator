# Secret Management

## Auto-Generation

By default, the operator generates cryptographic secrets for values not explicitly provided:

```yaml
spec:
  secrets:
    autoGenerate:
      enabled: true      # default: true
```

Auto-generated values are stored in a Secret named `<instance>-generated-secrets`:

| Key | Purpose |
|---|---|
| `nextauth-secret` | NextAuth session encryption |
| `salt` | Encryption salt |
| `clickhouse-username` | Managed ClickHouse username |
| `clickhouse-password` | Managed ClickHouse password |
| `redis-password` | Managed Redis password |
| `database-url` | Managed PostgreSQL connection string |

To provide your own values instead, set `secretRef` on the relevant spec fields. The operator skips auto-generation for any field with an explicit reference.

## Secret Rotation

The operator watches all Secrets referenced in the spec. When a Secret changes, it computes a hash annotation on the affected Deployment to trigger a rolling restart:

```yaml
spec:
  secrets:
    rotation:
      enabled: true     # default: true
```

Built-in mappings determine which components restart:

| Secret Type | Restarts |
|---|---|
| NextAuth / Salt / OIDC | Web |
| Redis | Web + Worker |
| ClickHouse | Web + Worker |
| Database | Web + Worker |
| Blob Storage | Worker |

### Custom Mappings

Add custom secret-to-component mappings:

```yaml
spec:
  secrets:
    rotation:
      enabled: true
      customMappings:
        - secretName: custom-api-key
          restartComponents:
            - web
            - worker
```

## How It Works

1. The Secret Controller watches all Secrets referenced by the `LangfuseInstance` spec
2. On change, it computes a SHA-256 hash of the relevant Secret data
3. The hash is stored as an annotation on the affected Deployment's pod template:
   ```
   langfuse.palena.ai/secret-hash: <sha256>
   ```
4. Kubernetes detects the annotation change and triggers a rolling update
