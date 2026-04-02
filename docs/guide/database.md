# Database (PostgreSQL)

Langfuse uses PostgreSQL as its primary data store. The operator supports three modes: **CloudNativePG**, **managed**, and **external**.

::: tip
Exactly one of `cloudnativepg`, `managed`, or `external` must be configured.
:::

## CloudNativePG

Reference an existing [CloudNativePG](https://cloudnative-pg.io/) cluster:

```yaml
spec:
  database:
    cloudnativepg:
      clusterRef:
        name: pg-cluster
        namespace: databases    # optional, defaults to instance namespace
      database: langfuse        # default: langfuse
```

The operator reads connection credentials from the CNPG-generated secret `<cluster-name>-app`.

## External

Connect to any PostgreSQL instance by providing a connection URL in a Secret:

```yaml
spec:
  database:
    external:
      secretRef:
        name: langfuse-db
        keys:
          url: database_url
          directUrl: direct_url   # optional, bypasses connection pooling
```

The Secret should contain the full PostgreSQL connection string:

```
postgresql://user:password@host:5432/langfuse?sslmode=require
```

The optional `directUrl` key is used for operations that need to bypass connection poolers like PgBouncer (e.g., migrations).

## Managed

The operator can deploy a PostgreSQL instance for you:

```yaml
spec:
  database:
    managed:
      instances: 3
      storageSize: "50Gi"
      storageClass: gp3-encrypted
      backup:
        enabled: true
        schedule: "0 2 * * *"
```

::: warning
Managed mode provisions a basic PostgreSQL StatefulSet. For production workloads, CloudNativePG or an external managed database is recommended.
:::

## Migrations

Database migrations are configured under `database.migration`:

```yaml
spec:
  database:
    migration:
      runOnDeploy: true            # run migrations on every reconcile (default: true)
      backgroundMigrations:
        enabled: true              # monitor background migrations (default: true)
        timeout: "3600s"           # max wait time
```

When `runOnDeploy` is true, the operator creates a Kubernetes Job with the new image to run `prisma migrate deploy` before updating the Web and Worker Deployments.
