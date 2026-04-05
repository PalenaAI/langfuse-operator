# ClickHouse

Langfuse uses ClickHouse for high-performance analytics and trace storage. The operator supports **managed** and **external** modes.

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

Deploy ClickHouse via the [Altinity ClickHouse Operator](https://github.com/Altinity/clickhouse-operator):

```yaml
spec:
  clickhouse:
    managed:
      shards: 1
      replicas: 3
      storageSize: "100Gi"
      storageClass: gp3-encrypted
      resources:
        preset: large              # small | medium | large | custom
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
| `small` | 1 | 4Gi |
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
