# LangfuseInstance

`langfuseinstances.langfuse.palena.ai/v1alpha1`

Deploys and manages the complete Langfuse stack: Web, Worker, and all dependent services.

## Spec

| Field | Type | Default | Description |
|---|---|---|---|
| `image` | [`ImageSpec`](#imagespec) | | Container image configuration |
| `web` | [`WebSpec`](#webspec) | | Web component configuration |
| `worker` | [`WorkerSpec`](#workerspec) | | Worker component configuration |
| `auth` | [`AuthSpec`](#authspec) | | Authentication configuration |
| `secrets` | [`SecretManagementSpec`](#secretmanagementspec) | | Secret generation and rotation |
| `database` | [`DatabaseSpec`](#databasespec) | | PostgreSQL configuration |
| `clickhouse` | [`ClickHouseSpec`](#clickhousespec) | | ClickHouse configuration |
| `redis` | [`RedisSpec`](#redisspec) | | Redis configuration |
| `blobStorage` | [`BlobStorageSpec`](#blobstoragespec) | | Blob storage configuration |
| `llm` | [`LLMSpec`](#llmspec) | | LLM integration |
| `ingress` | [`IngressSpec`](#ingressspec) | | Kubernetes Ingress |
| `route` | [`RouteSpec`](#routespec) | | OpenShift Route |
| `security` | [`SecuritySpec`](#securityspec) | | Security settings |
| `observability` | [`ObservabilitySpec`](#observabilityspec) | | Monitoring and tracing |
| `circuitBreaker` | [`CircuitBreakerSpec`](#circuitbreakerspec) | | Dependency circuit breaking |
| `upgrade` | [`UpgradeSpec`](#upgradespec) | | Upgrade strategy |

## Status

| Field | Type | Description |
|---|---|---|
| `ready` | bool | Whether the instance is fully operational |
| `phase` | string | `Pending`, `Migrating`, `Running`, `Degraded`, or `Error` |
| `web` | ComponentStatus | Web component state |
| `worker` | WorkerComponentStatus | Worker component state |
| `database` | DatabaseStatus | Database connection and migration state |
| `clickhouse` | ClickHouseStatus | ClickHouse state including storage |
| `redis` | ConnectionStatus | Redis connection state |
| `blobStorage` | BlobStorageStatus | Blob storage state |
| `secrets` | SecretsStatus | Secret management state |
| `version` | string | Currently running Langfuse version |
| `publicUrl` | string | Public URL of the instance |
| `conditions` | []Condition | Standard Kubernetes conditions |

### Conditions

| Type | Description |
|---|---|
| `Ready` | All components are operational |
| `DatabaseReady` | PostgreSQL is connected and migrated |
| `ClickHouseReady` | ClickHouse is connected |
| `RedisReady` | Redis is connected |
| `BlobStorageReady` | Blob storage is accessible |
| `MigrationsComplete` | All migrations have finished |
| `SecretsReady` | All secrets are generated/available |
| `ClickHouseRetentionApplied` | TTL policies are active |
| `ClickHouseSchemaDrift` | Schema drift detected |
| `CircuitBreakerActive` | A circuit breaker is tripped |

---

## Type Reference

### ImageSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `repository` | string | `langfuse/langfuse` | Container image repository |
| `tag` | string | **required** | Image tag |
| `pullPolicy` | string | `IfNotPresent` | `Always`, `IfNotPresent`, or `Never` |
| `pullSecrets` | []LocalObjectReference | | Image pull secrets |

### WebSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `replicas` | *int32 | `1` | Number of Web pod replicas |
| `autoscaling` | *AutoscalingSpec | | HPA configuration |
| `resources` | *ResourceRequirements | | CPU/memory requests and limits |
| `podDisruptionBudget` | *PDBSpec | | PDB configuration |
| `topologySpreadConstraints` | *TopologySpreadSpec | | Topology spread |
| `extraEnv` | []EnvVar | | Additional environment variables |
| `extraVolumeMounts` | []VolumeMount | | Additional volume mounts |
| `extraVolumes` | []Volume | | Additional volumes |
| `nodeSelector` | map[string]string | | Node selector |
| `tolerations` | []Toleration | | Tolerations |
| `affinity` | *Affinity | | Affinity rules |

### WorkerSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `replicas` | *int32 | `1` | Number of Worker pod replicas |
| `autoscaling` | *AutoscalingSpec | | HPA configuration |
| `resources` | *ResourceRequirements | | CPU/memory requests and limits |
| `concurrency` | *int32 | `10` | `LANGFUSE_WORKER_CONCURRENCY` |
| `extraEnv` | []EnvVar | | Additional environment variables |
| `nodeSelector` | map[string]string | | Node selector |
| `tolerations` | []Toleration | | Tolerations |
| `affinity` | *Affinity | | Affinity rules |

### AuthSpec

| Field | Type | Description |
|---|---|---|
| `nextAuthUrl` | string | Canonical URL for NextAuth (`NEXTAUTH_URL`) |
| `nextAuthSecret` | *SecretValue | Secret reference or auto-generate |
| `salt` | *SecretValue | Encryption salt reference or auto-generate |
| `emailPassword` | *EmailPasswordSpec | Email/password auth settings |
| `oidc` | *OIDCSpec | OpenID Connect settings |
| `initUser` | *InitUserSpec | Initial admin user |

### DatabaseSpec

| Field | Type | Description |
|---|---|---|
| `cloudnativepg` | *CloudNativePGSpec | Reference a CNPG Cluster |
| `managed` | *ManagedDatabaseSpec | Operator-managed PostgreSQL |
| `external` | *ExternalDatabaseSpec | External PostgreSQL |
| `migration` | *MigrationSpec | Migration behavior |

### ClickHouseSpec

| Field | Type | Description |
|---|---|---|
| `managed` | *ManagedClickHouseSpec | Managed via ClickHouse Operator |
| `external` | *ExternalClickHouseSpec | External ClickHouse |
| `encryption` | *ClickHouseEncryptionSpec | Encryption settings |
| `retention` | *RetentionSpec | Data retention policies |
| `schemaDrift` | *SchemaDriftSpec | Schema drift detection |

### RedisSpec

| Field | Type | Description |
|---|---|---|
| `managed` | *ManagedRedisSpec | Operator-managed Redis |
| `external` | *ExternalRedisSpec | External Redis |

### BlobStorageSpec

| Field | Type | Description |
|---|---|---|
| `provider` | string | `s3`, `azure`, or `gcs` |
| `s3` | *S3Spec | S3-compatible storage config |
| `azure` | *AzureBlobSpec | Azure Blob Storage config |
| `gcs` | *GCSSpec | Google Cloud Storage config |

### SecuritySpec

| Field | Type | Default | Description |
|---|---|---|---|
| `readOnlyRootFilesystem` | *bool | `true` | Read-only root filesystem |
| `runAsNonRoot` | *bool | `true` | Run containers as non-root |
| `networkPolicy.enabled` | *bool | `true` | Create NetworkPolicy |
| `telemetry.enabled` | *bool | `true` | Langfuse telemetry |

### CircuitBreakerSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `true` | Enable circuit breakers |
| `clickhouse` | *ComponentCircuitBreakerSpec | | ClickHouse circuit breaker |
| `redis` | *ComponentCircuitBreakerSpec | | Redis circuit breaker |
| `database` | *ComponentCircuitBreakerSpec | | Database circuit breaker |

### UpgradeSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `strategy` | string | `rolling` | Upgrade strategy |
| `preUpgrade.runMigrations` | *bool | `true` | Run migrations before upgrade |
| `preUpgrade.backupDatabase` | bool | `false` | Trigger CNPG backup |
| `rollingUpdate.maxUnavailable` | *int32 | `0` | Max unavailable during update |
| `rollingUpdate.maxSurge` | *int32 | `1` | Max surge during update |
| `postUpgrade.runBackgroundMigrations` | *bool | `true` | Monitor background migrations |
| `postUpgrade.healthCheckTimeout` | string | `120s` | Health check timeout |
| `postUpgrade.autoRollback` | bool | `false` | Revert on health failure |

## Example

```yaml
apiVersion: langfuse.palena.ai/v1alpha1
kind: LangfuseInstance
metadata:
  name: production
  namespace: langfuse
spec:
  image:
    tag: "3"
  auth:
    nextAuthUrl: "https://langfuse.example.com"
  web:
    replicas: 3
  worker:
    replicas: 2
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

### Print Columns

```
NAME         PHASE     READY   VERSION   AGE
production   Running   true    3         5d
```
