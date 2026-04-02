# Upgrades

The operator handles Langfuse version upgrades automatically when you change `spec.image.tag`.

## Upgrade Sequence

1. Detect `image.tag` change
2. Set `status.phase = Migrating`
3. Optionally trigger a CNPG backup
4. Create a migration Job with the new image
5. Wait for migration Job to succeed
6. Rolling update Web Deployment
7. Rolling update Worker Deployment
8. Monitor background migrations
9. Run post-upgrade health checks
10. Set `status.phase = Running`

## Configuration

```yaml
spec:
  upgrade:
    strategy: rolling

    preUpgrade:
      runMigrations: true       # default: true
      backupDatabase: true      # trigger CNPG backup before upgrade

    rollingUpdate:
      maxUnavailable: 0         # default: 0
      maxSurge: 1               # default: 1

    postUpgrade:
      runBackgroundMigrations: true   # default: true
      healthCheckTimeout: "120s"      # default: 120s
      autoRollback: true              # revert on health check failure
```

## Auto-Rollback

When `autoRollback` is enabled and post-upgrade health checks fail within the timeout, the operator:

1. Reverts Web and Worker Deployments to the previous image
2. Sets the condition `UpgradeRolledBack`
3. Emits a Kubernetes Event with details

The database migration is **not** rolled back automatically. PostgreSQL migrations created by Langfuse are designed to be forward-compatible.

## Monitoring an Upgrade

Watch the instance status:

```bash
kubectl get langfuseinstance langfuse -n langfuse -w
```

```
NAME       PHASE       READY   VERSION   AGE
langfuse   Migrating   false   3.21.0    5d
langfuse   Running     true    3.22.0    5d
```

Check conditions for details:

```bash
kubectl describe langfuseinstance langfuse -n langfuse
```
