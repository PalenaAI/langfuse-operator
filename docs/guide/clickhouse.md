# ClickHouse

Langfuse uses ClickHouse for high-performance analytics and trace storage. The operator supports **external** and **managed** modes.

::: warning Production guidance
For production workloads use **external** ClickHouse — either a managed service (ClickHouse Cloud, Altinity.Cloud, Aiven) or a cluster deployed via the [Altinity ClickHouse Operator](https://github.com/Altinity/clickhouse-operator) that you operate yourself. Managed mode in this operator is a **single-node, dev-only** deployment with no replication, no sharding, and no backups.
:::

## External

Connect to an existing ClickHouse instance:

```yaml
spec:
  clickhouse:
    external:
      secretRef:
        name: langfuse-clickhouse
        keys:
          url: url                      # HTTP interface (http://host:8123)
          migrationUrl: migration_url   # native protocol (clickhouse://host:9000)
          username: username
          password: password
```

::: info
`migrationUrl` uses the ClickHouse **native protocol** (`clickhouse://host:9000`) and is required for schema migrations. `url` uses the **HTTP interface** (`http://host:8123`) for query traffic.
:::

::: tip
For single-node ClickHouse deployments, the operator automatically sets `CLICKHOUSE_CLUSTER_ENABLED=false` to avoid `ON CLUSTER` DDL errors that require ZooKeeper/Keeper.
:::

## Managed

::: danger Dev / preview only
Managed mode deploys a **plain single-node ClickHouse StatefulSet**, not a clustered deployment via the Altinity ClickHouse Operator. `CLICKHOUSE_CLUSTER_ENABLED=false` is forced — no ZooKeeper/Keeper, no `ReplicatedMergeTree`, no `ON CLUSTER` DDL. The operator does not take backups or snapshots. Suitable for local development, evaluation, and CI; **not for production**.

The `shards` field is ignored. Setting `replicas > 1` creates N independent pods that do not replicate data — do not use.
:::

Deploy a single-node ClickHouse for development:

```yaml
spec:
  clickhouse:
    managed:
      storageSize: "100Gi"
      storageClass: gp3-encrypted
      resources:
        preset: small              # small | medium | large | custom
      auth:
        secretRef:                 # optional, omit to auto-generate
          name: ch-creds
          keys:
            username: username
            password: password
```

Resource presets:

| Preset | CPU Request | Memory Request |
|---|---|---|
| `small` | 1 | 2Gi |
| `medium` | 2 | 8Gi |
| `large` | 4 | 16Gi |
| `custom` | user-defined | user-defined |

## Encryption

```yaml
spec:
  clickhouse:
    encryption:
      enabled: true        # encryption at rest
      blobStorage: false   # encrypt blob storage data
```

## Data Retention

Configure TTL-based retention per table type:

```yaml
spec:
  clickhouse:
    retention:
      traces:
        ttlDays: 90          # 0 = infinite
      observations:
        ttlDays: 90
      scores:
        ttlDays: 180
      storagePressure:
        enabled: true
        warningThresholdPercent: 75
        criticalThresholdPercent: 90
        pruneOldestPartitions: true
        minRetainDays: 7
```

When storage pressure exceeds the critical threshold, the operator prunes the oldest partitions while respecting `minRetainDays`.

## Schema Drift Detection

The operator periodically validates the ClickHouse schema against what Langfuse expects:

```yaml
spec:
  clickhouse:
    schemaDrift:
      enabled: true
      checkIntervalMinutes: 60
      autoRepair: false        # set to true to auto-fix drift
```

When drift is detected, the operator sets a `ClickHouseSchemaDrift` status condition and emits an event. With `autoRepair: true`, it attempts to apply corrective DDL.
