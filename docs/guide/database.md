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

## Managed (deprecated)

::: danger Rejected since 0.10.0, removed in 0.11.0
`database.managed` was never implemented — the operator does not deploy PostgreSQL in this mode, and it wired `DATABASE_URL` to a Secret key nothing creates, so the pods could only fail with `CreateContainerConfigError`.

Since **0.10.0** the operator rejects it outright with a `Ready=False` / `ConfigError` condition rather than producing broken pods. It is removed entirely in **0.11.0**.

Use [`cloudnativepg`](#cloudnativepg) for an operator-managed HA PostgreSQL, or [`external`](#external) for a managed service.
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
