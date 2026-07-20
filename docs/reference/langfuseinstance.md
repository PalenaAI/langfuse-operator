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
| `tls` | [`TLSSpec`](#tlsspec) | | Trusted CA for encrypted datastore connections; see [Datastore TLS](../guide/datastore-tls.md) |
| `eeLicenseKey` | *SecretValue | | `LANGFUSE_EE_LICENSE_KEY` reference. Required for the `LangfuseOrganization`/`LangfuseProject` CRDs (EE/Pro-gated org-management API); see [Multi-Tenancy](../guide/multi-tenancy.md) |
| `secrets` | [`SecretManagementSpec`](#secretmanagementspec) | | Secret generation and rotation |
| `database` | [`DatabaseSpec`](#databasespec) | | PostgreSQL configuration |
| `clickhouse` | [`ClickHouseSpec`](#clickhousespec) | | ClickHouse configuration |
| `redis` | [`RedisSpec`](#redisspec) | | Redis configuration |
| `blobStorage` | [`BlobStorageSpec`](#blobstoragespec) | | Blob storage configuration |
| `llm` | [`LLMSpec`](#llmspec) | | LLM integration |
| `ingress` | [`IngressSpec`](#ingressspec) | | Kubernetes Ingress |
| `route` | [`RouteSpec`](#routespec) | | OpenShift Route |
| `gatewayAPI` | [`GatewayAPISpec`](#gatewayapispec) | | Gateway API HTTPRoute |
| `security` | [`SecuritySpec`](#securityspec) | | Security settings |
| `observability` | [`ObservabilitySpec`](#observabilityspec) | | Monitoring and tracing |
| `circuitBreaker` | [`CircuitBreakerSpec`](#circuitbreakerspec) | | Dependency circuit breaking |
| `upgrade` | [`UpgradeSpec`](#upgradespec) | | Upgrade strategy |

## Status

| Field | Type | Description |
|---|---|---|
| `ready` | bool | Whether the instance is fully operational |
| `phase` | string | `Pending`, `Migrating`, `Running`, `Degraded`, or `Error` |
| `web` | ComponentStatus | Web component state: `replicas`, `readyReplicas`, `endpoint`, and [`issues`](#podissue) |
| `worker` | WorkerComponentStatus | Worker component state: as above plus `queueDepth`, `circuitBreakerActive` |
| `database` | DatabaseStatus | Database connection and migration state |
| `migration` | [`MigrationStatus`](#migrationstatus) | Migration Job state, including pod-level failures |
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
| `extraVolumeMounts` | []VolumeMount | | Additional volume mounts on the Worker container |
| `extraVolumes` | []Volume | | Additional volumes on the Worker pod |
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
| `adminApiKey` | *SecretValue | `ADMIN_API_KEY` reference or auto-generate; used by the Organization/Project controllers (see [Multi-Tenancy](../guide/multi-tenancy.md)) |

### DatabaseSpec

| Field | Type | Description |
|---|---|---|
| `cloudnativepg` | *CloudNativePGSpec | Reference a CNPG Cluster (recommended for production) |
| `external` | *ExternalDatabaseSpec | External PostgreSQL (recommended for production) |
| `managed` | *ManagedDatabaseSpec | **Not implemented** — reserved for a future release |
| `migration` | *MigrationSpec | Migration behavior |

### ClickHouseSpec

| Field | Type | Description |
|---|---|---|
| `external` | *ExternalClickHouseSpec | External ClickHouse (recommended for production) |
| `managed` | *ManagedClickHouseSpec | Single-node StatefulSet (dev / preview only — no replication, no backups) |
| `encryption` | *ClickHouseEncryptionSpec | Encryption settings |
| `retention` | *RetentionSpec | Data retention policies |
| `schemaDrift` | *SchemaDriftSpec | Schema drift detection |

### RedisSpec

| Field | Type | Description |
|---|---|---|
| `external` | *ExternalRedisSpec | External Redis (recommended for production) |
| `managed` | *ManagedRedisSpec | Single-pod StatefulSet (dev / preview only — no HA, no backups) |

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
| `preUpgrade` | [`*PreUpgradeSpec`](#preupgradespec) | | Actions before upgrade |
| `rollingUpdate` | [`*RollingUpdateSpec`](#rollingupdatespec) | | Rolling update parameters |
| `postUpgrade` | [`*PostUpgradeSpec`](#postupgradespec) | | Actions after upgrade |

---

### Nested types

The remaining `*Spec` types referenced above. Field defaults marked `*T` mean the field is a pointer (omitting it falls back to the default; setting it explicitly to the zero value sticks).

### SecretValue

| Field | Type | Description |
|---|---|---|
| `secretRef` | *SecretKeyRef | Reference to an existing Secret key. When nil and auto-generation is enabled, the operator generates the value. |

### AutoscalingSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Toggle HPA creation |
| `minReplicas` | *int32 | `1` | Lower bound |
| `maxReplicas` | int32 | `10` | Upper bound |
| `targetCPUUtilization` | *int32 | `80` | Target CPU utilization (%) |
| `customMetrics` | []CustomMetric | | Additional scaling metrics (`type`, `threshold`) |

### PDBSpec

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Toggle PDB creation |
| `minAvailable` | *int32 | Minimum pods that must remain available |

### TopologySpreadSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Toggle topology spread constraints |
| `maxSkew` | *int32 | `1` | Maximum spread skew |
| `topologyKey` | string | `topology.kubernetes.io/zone` | Topology domain key |

### EmailPasswordSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `true` | Toggle email/password auth |
| `disableSignup` | bool | `false` | Block new user registration |

### OIDCSpec

Configures Langfuse's generic custom OIDC provider (mapped to the upstream `AUTH_CUSTOM_*` variables). The IdP must whitelist the callback URL `<NEXTAUTH_URL>/api/auth/callback/custom`.

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Toggle OIDC |
| `issuer` | string | OIDC issuer URL → `AUTH_CUSTOM_ISSUER` |
| `clientId` | *SecretKeyRef | Reference to OIDC client ID → `AUTH_CUSTOM_CLIENT_ID` |
| `clientSecret` | *SecretKeyRef | Reference to OIDC client secret → `AUTH_CUSTOM_CLIENT_SECRET` |
| `name` | string | Login button label → `AUTH_CUSTOM_NAME` (default `SSO`) |
| `scope` | []string | Requested OAuth scopes → `AUTH_CUSTOM_SCOPE`, space-joined (default `openid email profile`) |
| `ssoEnforcedDomains` | []string | Domains forced to sign in via SSO → `AUTH_DOMAINS_WITH_SSO_ENFORCEMENT`, comma-joined (password login disabled for them) |

### InitUserSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Create the initial admin user on first boot |
| `email` | string | | Initial user email |
| `password` | *SecretKeyRef | | Reference to the initial password |
| `orgName` | string | `Default` | Default organization name |
| `projectName` | string | `Default` | Default project name |

### SecretManagementSpec

| Field | Type | Description |
|---|---|---|
| `autoGenerate` | [`*AutoGenerateSpec`](#autogeneratespec) | Auto-generation of `NEXTAUTH_SECRET`, `SALT`, etc. |
| `rotation` | [`*RotationSpec`](#rotationspec) | Secret-rotation detection and restart |

### AutoGenerateSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `true` | Toggle auto-generation of operator-owned secrets |

### RotationSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `true` | Detect secret changes and trigger component restarts |
| `customMappings` | []SecretRestartMapping | | Map a Secret name → components to restart (`secretName`, `restartComponents: [web, worker]`) |

### CloudNativePGSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `clusterRef` | ObjectReference | | Reference to an existing CNPG `Cluster` |
| `database` | string | `langfuse` | Database name within the cluster |

### ManagedDatabaseSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `instances` | *int32 | `1` | Number of PostgreSQL instances |
| `storageSize` | string | `10Gi` | PVC size for each instance |
| `storageClass` | string | | Storage class for PVCs |
| `backup` | [`*DatabaseBackupSpec`](#databasebackupspec) | | Automated backup configuration |

### DatabaseBackupSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Toggle automated backups |
| `schedule` | string | `0 2 * * *` | Cron schedule |

### ExternalDatabaseSpec

| Field | Type | Description |
|---|---|---|
| `secretRef` | SecretKeysRef | Reference to a Secret with connection details. Recognised keys: `url` (required, `postgres://…`), `directUrl` (optional, bypasses pooling). With a `tls` block the `url` must **not** contain a query string. |
| `tls` | [`DatabaseTLSSpec`](#databasetlsspec) | TLS for the PostgreSQL connection. |

### MigrationSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `runOnDeploy` | *bool | `true` | Run migrations on every deployment |
| `backgroundMigrations` | [`*BackgroundMigrationSpec`](#backgroundmigrationspec) | | Background-migration handling |

### BackgroundMigrationSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `true` | Monitor background migrations via `/api/public/background-migrations` |
| `timeout` | string | `3600s` | Maximum wait |

### ManagedClickHouseSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `shards` | *int32 | `1` | Number of shards |
| `replicas` | *int32 | `1` | Replicas per shard |
| `storageSize` | string | `50Gi` | PVC size |
| `storageClass` | string | | Storage class |
| `resources` | [`*ClickHouseResourceSpec`](#clickhouseresourcespec) | | Resource preset or custom |
| `auth` | [`*ClickHouseAuthSpec`](#clickhouseauthspec) | | Credentials reference |

### ClickHouseResourceSpec

| Field | Type | Description |
|---|---|---|
| `preset` | string | One of `small`, `medium`, `large`, `custom` |
| `custom` | *ResourceRequirements | Used when `preset: custom` |

### ClickHouseAuthSpec

| Field | Type | Description |
|---|---|---|
| `secretRef` | *SecretKeysRef | Reference to a Secret with `username` and `password` keys. Omit to let the operator auto-generate. |

### ExternalClickHouseSpec

| Field | Type | Description |
|---|---|---|
| `secretRef` | SecretKeysRef | Reference to a Secret with connection details. Recognised keys: `url` (HTTP, e.g. `http://ch:8123`), `migrationUrl` (native, e.g. `clickhouse://ch:9000`), `username`, `password`. With a `tls` block, use the TLS scheme/port (`https://…:8443`, `clickhouse://…:9440`). |
| `tls` | [`ClickHouseTLSSpec`](#clickhousetlsspec) | TLS for the ClickHouse connection. |

### ClickHouseEncryptionSpec

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Encryption at rest |
| `blobStorage` | bool | Blob-storage encryption |

### RetentionSpec

| Field | Type | Description |
|---|---|---|
| `traces` | [`*TableRetentionSpec`](#tableretentionspec) | TTL for trace data |
| `observations` | [`*TableRetentionSpec`](#tableretentionspec) | TTL for observation data |
| `scores` | [`*TableRetentionSpec`](#tableretentionspec) | TTL for score data |
| `storagePressure` | [`*StoragePressureSpec`](#storagepressurespec) | Auto-retention under disk pressure |

### TableRetentionSpec

| Field | Type | Description |
|---|---|---|
| `ttlDays` | int32 | Days to retain data; `0` = infinite |

### StoragePressureSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Monitor ClickHouse storage pressure |
| `warningThresholdPercent` | int32 | `75` | Emit a warning event above this |
| `criticalThresholdPercent` | int32 | `90` | Begin pruning above this |
| `pruneOldestPartitions` | bool | `false` | Drop oldest partitions when critical |
| `minRetainDays` | int32 | `7` | Floor for retention even under pressure |

### SchemaDriftSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Periodic schema drift detection |
| `checkIntervalMinutes` | int32 | `60` | Interval between checks |
| `autoRepair` | bool | `false` | Automatically repair detected drift |

### ManagedRedisSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `replicas` | *int32 | `1` | Number of Redis replicas |
| `storageSize` | string | `5Gi` | PVC size |

### ExternalRedisSpec

| Field | Type | Description |
|---|---|---|
| `secretRef` | SecretKeysRef | Reference to a Secret with connection details. Recognised keys: `host`, `port`, `password`, `tls` (legacy boolean; prefer the `tls` block). |
| `tls` | [`RedisTLSSpec`](#redistlsspec) | TLS for the Redis connection. |

### TLSSpec

Trust configuration for encrypted datastore connections. See [Datastore TLS](../guide/datastore-tls.md).

| Field | Type | Description |
|---|---|---|
| `trustedCASecretRef` | [`CACertSecretRef`](#cacertsecretref) | CA mounted into Web + Worker and exported as `NODE_EXTRA_CA_CERTS`. Covers ClickHouse HTTPS, and is the default CA for Redis/PostgreSQL. |

### DatabaseTLSSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `sslMode` | string | `require` | `disable`, `require`, `verify-ca`, or `verify-full`. Mapped to Prisma's `sslmode`/`sslaccept` parameters (Prisma has no CA-only mode, so `verify-ca` ≡ `verify-full`). |
| `caSecretRef` | [`CACertSecretRef`](#cacertsecretref) | | CA used as Prisma's `sslcert`. Defaults to `spec.tls.trustedCASecretRef`. |

The operator composes `DATABASE_URL` as `$(DATABASE_URL_BASE)?<params>` via env interpolation, so the `url` in the Secret must not contain its own query string.

### ClickHouseTLSSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Sets `CLICKHOUSE_MIGRATION_SSL=true`. The runtime HTTPS client trusts the CA via `NODE_EXTRA_CA_CERTS`. URLs in the Secret must use the TLS scheme/port. |

### RedisTLSSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Sets `REDIS_TLS_ENABLED=true` on Web + Worker. |
| `caSecretRef` | [`CACertSecretRef`](#cacertsecretref) | | CA for `REDIS_TLS_CA_PATH`. Defaults to `spec.tls.trustedCASecretRef` (ioredis ignores `NODE_EXTRA_CA_CERTS`). |
| `clientCertSecretRef` | [`ClientCertSecretRef`](#clientcertsecretref) | | Client cert/key for mutual TLS (`REDIS_TLS_CERT_PATH` / `REDIS_TLS_KEY_PATH`). |
| `serverName` | string | | TLS SNI/hostname override (`REDIS_TLS_SERVERNAME`). |

### CACertSecretRef

| Field | Type | Default | Description |
|---|---|---|---|
| `name` | string | | Secret name. |
| `key` | string | `ca.crt` | Secret key holding the PEM CA certificate. |

### ClientCertSecretRef

| Field | Type | Default | Description |
|---|---|---|---|
| `name` | string | | Secret name. |
| `certKey` | string | `tls.crt` | Secret key holding the PEM client certificate. |
| `keyKey` | string | `tls.key` | Secret key holding the PEM client private key. |

### PodIssue

A pod-level failure surfaced into `status.web.issues`, `status.worker.issues`, or `status.migration.issues`, so a stuck component can be diagnosed without inspecting pods by hand. Populated only while a component is not ready.

| Field | Type | Description |
|---|---|---|
| `pod` | string | Name of the affected pod. |
| `container` | string | Container that reported the problem. Empty for pod-level problems such as scheduling failures. |
| `reason` | string | Kubernetes reason, e.g. `CrashLoopBackOff`, `ImagePullBackOff`, `CreateContainerConfigError`, `Unschedulable`, `OOMKilled`. |
| `message` | string | Human-readable detail. For a crash loop this includes the previous run's exit code and captured output. |
| `restartCount` | int32 | Container restart count. |
| `fatal` | bool | The failure cannot self-heal and needs human action. Any fatal issue moves the instance to `phase: Error` instead of `Degraded`. |

**Fatal** reasons are those that never resolve on their own: `ImagePullBackOff`, `ErrImagePull`, `InvalidImageName`, `ErrImageNeverPull`, `CreateContainerConfigError`, `CreateContainerError`. `CrashLoopBackOff` is deliberately **not** fatal — Langfuse containers legitimately crash-loop while waiting for Postgres or ClickHouse during a cold start.

### MigrationStatus

| Field | Type | Description |
|---|---|---|
| `jobName` | string | Name of the migration Job. |
| `failed` | int32 | Number of failed migration pod attempts. |
| `issues` | [][`PodIssue`](#podissue) | Pod-level problems from the migration Job's pods. |

### S3Spec

| Field | Type | Description |
|---|---|---|
| `endpoint` | string | S3 endpoint URL (set for MinIO; omit for AWS) |
| `region` | string | S3 region |
| `bucket` | string | Bucket name (required) |
| `forcePathStyle` | bool | Path-style addressing (MinIO) |
| `credentials` | [`*S3CredentialsSpec`](#s3credentialsspec) | Credentials reference |

### S3CredentialsSpec

| Field | Type | Description |
|---|---|---|
| `secretRef` | SecretKeysRef | Reference to a Secret with `accessKeyId` and `secretAccessKey` |

### AzureBlobSpec

| Field | Type | Description |
|---|---|---|
| `storageAccountName` | string | Azure storage account name (used as the access key ID and to derive the default endpoint) |
| `containerName` | string | Blob container name (Langfuse's upload "bucket") |
| `endpoint` | string | Blob service endpoint override. Defaults to `https://<storageAccountName>.blob.core.windows.net` |
| `credentials` | [`*AzureCredentialsSpec`](#azurecredentialsspec) | Credentials reference |

### AzureCredentialsSpec

| Field | Type | Description |
|---|---|---|
| `secretRef` | SecretKeysRef | Reference to a Secret holding the storage **account key** under the `accountKey` key (override via the Keys map). Langfuse does not support Azure connection strings. |

### GCSSpec

| Field | Type | Description |
|---|---|---|
| `bucketName` | string | GCS bucket name |
| `projectId` | string | GCP project ID |
| `credentials` | [`*GCSCredentialsSpec`](#gcscredentialsspec) | Credentials reference |

### GCSCredentialsSpec

| Field | Type | Description |
|---|---|---|
| `secretRef` | SecretKeysRef | Reference to a Secret containing the GCP service-account JSON |

### LLMSpec

| Field | Type | Description |
|---|---|---|
| `apiBase` | string | LLM API base URL |
| `apiKey` | *SecretKeyRef | Reference to the LLM API key |
| `model` | string | LLM model name |

### IngressSpec

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Toggle Ingress creation |
| `className` | string | `IngressClass` name |
| `host` | string | Ingress hostname |
| `annotations` | map[string]string | Additional Ingress annotations |
| `tls` | [`*IngressTLSSpec`](#ingresstlsspec) | TLS configuration |

### IngressTLSSpec

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Toggle TLS |
| `secretName` | string | Existing TLS Secret name |
| `certManager` | [`*CertManagerSpec`](#certmanagerspec) | cert-manager integration |

### CertManagerSpec

| Field | Type | Description |
|---|---|---|
| `issuerRef.name` | string | Issuer name |
| `issuerRef.kind` | string | `Issuer` or `ClusterIssuer` (default `ClusterIssuer`) |

### RouteSpec

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Toggle OpenShift Route creation |
| `host` | string | Route hostname |
| `annotations` | map[string]string | Additional Route annotations |

### GatewayAPISpec

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Toggle HTTPRoute creation |
| `gatewayRef.name` | string | Gateway name (required) |
| `gatewayRef.namespace` | string | Gateway namespace (default: HTTPRoute namespace) |
| `gatewayRef.sectionName` | string | Listener name on the Gateway |
| `hostname` | string | HTTP hostname to match |
| `annotations` | map[string]string | Additional HTTPRoute annotations |

### NetworkPolicySpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `true` | Create per-component NetworkPolicies |
| `extraEgressPorts` | []NetworkPolicyPort | | Additional destination ports to allow. The defaults cover the well-known datastore ports (plaintext **and** TLS); use this for non-standard ports such as a connection pooler. See [Networking](../guide/networking.md#non-standard-ports). |

### NetworkPolicyPort

| Field | Type | Default | Description |
|---|---|---|---|
| `port` | int32 | | Destination port (1–65535). |
| `protocol` | string | `TCP` | `TCP` or `UDP`. |

### TelemetrySpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `true` | Toggle Langfuse's built-in telemetry (`TELEMETRY_ENABLED`) |

### ObservabilitySpec

| Field | Type | Description |
|---|---|---|
| `serviceMonitor` | [`*ServiceMonitorSpec`](#servicemonitorspec) | Prometheus ServiceMonitor |
| `otel` | [`*OTELSpec`](#otelspec) | OpenTelemetry integration |

### ServiceMonitorSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Create a Prometheus ServiceMonitor |
| `interval` | string | `30s` | Scrape interval |
| `labels` | map[string]string | | Additional ServiceMonitor labels |

### OTELSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Toggle OTEL |
| `endpoint` | string | | OTEL collector endpoint |
| `protocol` | string | `grpc` | `grpc` or `http` |

### ComponentCircuitBreakerSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `action` | string | | `scaleWorkerToZero`, `emitCriticalEvent`, or `none` |
| `probeIntervalSeconds` | int32 | `15` | Health probe interval |
| `failureThreshold` | int32 | `3` | Failures before opening the circuit |
| `recoveryAction` | string | | `restoreScale` or `none` |

### PreUpgradeSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `runMigrations` | *bool | `true` | Run migrations before upgrade |
| `backupDatabase` | bool | `false` | Trigger a CNPG backup |

### RollingUpdateSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `maxUnavailable` | *int32 | `0` | Max unavailable pods during update |
| `maxSurge` | *int32 | `1` | Max extra pods during update |

### PostUpgradeSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `runBackgroundMigrations` | *bool | `true` | Monitor background migrations after upgrade |
| `healthCheckTimeout` | string | `120s` | Timeout for post-upgrade health checks |
| `autoRollback` | bool | `false` | Revert on health failure |

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
