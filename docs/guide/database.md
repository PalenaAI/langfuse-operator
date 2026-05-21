# Database (PostgreSQL)

Langfuse uses PostgreSQL as its primary data store. The operator supports three modes: **CloudNativePG**, **external**, and **managed**.

::: tip
Exactly one of `cloudnativepg`, `external`, or `managed` must be configured.
:::

::: warning Production guidance
For production workloads use **CloudNativePG** (HA, streaming replication, point-in-time recovery, barman/pgbackrest backups) or an **external** managed Postgres (RDS, Cloud SQL, Aiven, etc.). The operator does not manage HA or backups for any other Postgres deployment.
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

::: danger Not implemented
Managed Postgres is reserved in the CRD schema but **not implemented** in the current release. Setting `database.managed` will wire `DATABASE_URL` to a Secret the operator does not create — Langfuse will fail to start. Use `cloudnativepg` or `external` instead.

The `instances`, `storageSize`, `storageClass`, and `backup` fields are accepted by validation but ignored at reconcile time. They will be honored once managed Postgres ships (no committed timeline).
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
