# Changelog

All notable changes to the Langfuse Operator will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Pod-level failures are now reported on the LangfuseInstance.** Previously a component that could not start showed up only as `Phase: Pending` with a `0/1 ready replicas` message, and the actual cause — `CreateContainerConfigError` from a missing Secret key, `ImagePullBackOff`, `CrashLoopBackOff`, `OOMKilled`, `Unschedulable` — was visible only by inspecting pods by hand. The operator now surfaces it directly:
  - **`status.web.issues` / `status.worker.issues` / `status.migration.issues`** list the offending pod, container, Kubernetes reason, message, and restart count. For a crash loop the message includes the previous run's exit code and captured output (so a Langfuse `ZodError` about a missing env var is visible in the CR).
  - The `WebReady` / `WorkerReady` / `MigrationsComplete` conditions now carry the real reason and detail instead of a bare replica count, so `kubectl describe` explains the failure.
  - **New `Error` phase semantics** — failures that cannot self-heal (bad image reference, missing Secret key) report `Phase: Error` instead of sitting in `Pending`/`Degraded` indefinitely. `CrashLoopBackOff` stays non-fatal, since Langfuse containers legitimately crash-loop while waiting for Postgres/ClickHouse on a cold start.
  - **`status.migration`** reports the migration Job name, failed attempt count, and its pods' issues.
  - See [docs/guide/troubleshooting.md](docs/guide/troubleshooting.md).
- **`spec.security.networkPolicy.extraEgressPorts`** — allow additional destination ports in the generated egress rules, for datastores on non-standard ports (connection poolers, cloud provider ports) or sidecars.

### Fixed

- **The default NetworkPolicy no longer blocks encrypted datastore connections.** Egress was restricted to the plaintext ports only (5432/8123/9000/6379/443/3000), so enabling the datastore TLS introduced in 0.9.0 — which the documentation configures on `https://…:8443`, `clickhouse://…:9440`, or Redis on 6380 — had its traffic silently dropped by the operator's own policy, surfacing as an opaque connection timeout. Since `spec.security.networkPolicy` defaults to **enabled**, this affected every instance using datastore TLS. Egress now covers both the plaintext and TLS ports of each store.
- **Health checks no longer skip instances stuck in `Pending`.** The health monitor only ran when the instance was already `Running` or `Degraded`, so a first rollout that never came up — precisely when the dependency probes and pod diagnostics are most useful — was never evaluated and the instance sat in `Pending` with no explanation. Health checks now run in `Pending` too (they are still skipped during `Migrating`, which the migration controller owns).

## [0.9.0] - 2026-06-28

### Added

- **Datastore TLS — encrypt Langfuse's connections to PostgreSQL, ClickHouse, and Redis.** New TLS configuration that applies to **both** the Web and Worker components (and the migration Job), so encryption holds across the whole data plane — the Worker does most of the Redis/ClickHouse work. See [docs/guide/datastore-tls.md](docs/guide/datastore-tls.md).
  - **`spec.tls.trustedCASecretRef`** — mounts a caller-supplied CA (e.g. a cert-manager `ca.crt`) into the pods and exports `NODE_EXTRA_CA_CERTS`, making the Node.js runtime trust it for all outbound TLS. This alone covers ClickHouse HTTPS and is the default CA for the Redis/PostgreSQL connections.
  - **`spec.redis.external.tls`** — `enabled`, optional `caSecretRef`, `clientCertSecretRef` (mutual TLS), and `serverName` (SNI). Maps to `REDIS_TLS_ENABLED` / `REDIS_TLS_CA_PATH` / `REDIS_TLS_CERT_PATH` / `REDIS_TLS_KEY_PATH` / `REDIS_TLS_SERVERNAME`. The CA path is always set because Langfuse's Redis client does not read the Node trust store.
  - **`spec.clickhouse.external.tls.enabled`** — sets `CLICKHOUSE_MIGRATION_SSL=true`; provide the `https://…:8443` / `clickhouse://…:9440` URLs in the connection Secret.
  - **`spec.database.external.tls`** — `sslMode` (`disable`/`require`/`verify-ca`/`verify-full`) and optional `caSecretRef`. The operator composes `DATABASE_URL` with Prisma's TLS parameters (`sslmode`/`sslaccept`/`sslcert`) via env interpolation, so the connection URL in the Secret must not contain a query string.
- **`spec.worker.extraVolumes` / `spec.worker.extraVolumeMounts`** — parity with `spec.web`, so arbitrary volumes (extra certificates, etc.) can be mounted into the Worker pod as a general escape hatch.

## [0.8.0] - 2026-06-25

### Fixed

- **OIDC/SSO via `spec.auth.oidc` now actually works.** The operator emitted `AUTH_OIDC_ENABLED` / `AUTH_OIDC_ISSUER` / `AUTH_OIDC_CLIENT_ID` / `AUTH_OIDC_CLIENT_SECRET`, none of which exist in Langfuse v3 — so configuring `spec.auth.oidc` produced no working single sign-on. The operator now configures Langfuse's generic custom OIDC provider with the correct `AUTH_CUSTOM_*` variables (`AUTH_CUSTOM_CLIENT_ID`, `AUTH_CUSTOM_CLIENT_SECRET`, `AUTH_CUSTOM_ISSUER`, `AUTH_CUSTOM_NAME`, `AUTH_CUSTOM_SCOPE`). Whitelist the callback URL `<NEXTAUTH_URL>/api/auth/callback/custom` in your identity provider.

### Added

- **`spec.auth.oidc.name`** — sets the SSO login-button label in the Langfuse UI (`AUTH_CUSTOM_NAME`, defaults to `SSO`).
- **`spec.auth.oidc.scope`** — list of OAuth scopes requested from the provider (`AUTH_CUSTOM_SCOPE`, space-joined, defaults to `openid email profile`).

### Changed

- **`spec.auth.oidc.allowedDomains` renamed to `spec.auth.oidc.ssoEnforcedDomains`.** The previous field mapped to a non-existent variable and did nothing. It now maps to the upstream `AUTH_DOMAINS_WITH_SSO_ENFORCEMENT` setting: the listed domains may only sign in via SSO and password login is disabled for them. Note this is a global SSO-enforcement setting, not a per-provider allow-list — Langfuse has no generic custom-OIDC allowed-domains variable.

## [0.7.1] - 2026-06-05

### Fixed

- **The Worker now runs the dedicated queue-consumer image, so ingestion actually drains.** The `langfuse-worker` Deployment was started from the main `langfuse/langfuse` image — which only serves the web app/API and never runs the BullMQ workers — so events POSTed to `/api/public/ingestion` piled up in Redis and never reached ClickHouse, leaving Tracing and Sessions empty for every project. The Worker now uses `langfuse/langfuse-worker` with the same tag as `spec.image`, while the Web component keeps using `langfuse/langfuse`.
- **Azure Blob Storage and Google Cloud Storage now actually work.** The operator emitted `LANGFUSE_BLOB_STORAGE_PROVIDER` / `LANGFUSE_AZURE_*` / `LANGFUSE_GCS_*` env vars that Langfuse v3 does not read, and never set the required `LANGFUSE_S3_EVENT_UPLOAD_BUCKET` — so the pods crashed on startup with a `ZodError` (`LANGFUSE_S3_EVENT_UPLOAD_BUCKET … expected string, received undefined`). Langfuse v3 reuses the S3 event-upload settings for all providers, toggled by `LANGFUSE_USE_AZURE_BLOB` / `LANGFUSE_USE_GOOGLE_CLOUD_STORAGE`; the operator now generates the correct variables (container → bucket, account name → access key ID, account key → secret access key, derived blob endpoint for Azure).

### Added

- **`spec.worker.image`** — override the Worker container image repository and tag (defaults to `langfuse/langfuse-worker` at `spec.image.tag`) for custom registries or mirrors.
- **`spec.blobStorage.azure.endpoint`** — override the Azure blob service endpoint (for Azure Government, sovereign clouds, or Azurite). Defaults to `https://<storageAccountName>.blob.core.windows.net`.

### Changed

- **Azure Blob Storage requires the storage account key, not a connection string.** Provide the account key in the referenced Secret under the `accountKey` key (Langfuse v3 has no connection-string support). GCS credentials are read from the `credentials` Secret key (inline service-account JSON), or omit the credentials block to use GKE Workload Identity.

## [0.7.0] - 2026-05-29

### Fixed

- **`LangfuseOrganization` / `LangfuseProject` CRDs now actually work (against a licensed Langfuse).** They were non-functional: the controllers authenticated to the Langfuse admin API with the wrong scheme (Basic public/secret keys instead of the `ADMIN_API_KEY` Bearer token) and read a `<instance>-operator-credentials` Secret that nothing ever created or documented — so every reconcile failed with a missing-secret error. The operator now:
  - Provisions an `ADMIN_API_KEY` (auto-generated, or from `spec.auth.adminApiKey`), injects it into the Langfuse containers, and authenticates the admin API with it as a Bearer token.
  - Manages **projects** through Langfuse's public API using an organization-scoped key minted via the admin API and cached in a dedicated owned Secret (`<org>-orgkey`) — the admin API has no project endpoints, which the previous implementation incorrectly assumed.
- **Auto-generated secret backfills missing keys (upgrade safety).** The operator previously skipped an existing `<instance>-generated-secrets` entirely, so upgrading to a version that adds a new generated key (such as `admin-api-key`) left the Secret without it and the Langfuse pods failed with `CreateContainerConfigError` on the missing env reference. The operator now backfills any missing operator-owned keys into the existing Secret while preserving current values.

### Added

- **`spec.auth.adminApiKey`** — reference or auto-generate the `ADMIN_API_KEY` used for organization/project management.
- **`spec.eeLicenseKey`** — reference a Langfuse self-hosted Enterprise/Pro license key (`LANGFUSE_EE_LICENSE_KEY`), injected into the Langfuse containers.

### Notes

- **The `LangfuseOrganization` and `LangfuseProject` CRDs require a Langfuse self-hosted Enterprise/Pro license.** Langfuse's organization-management API is gated behind the `admin-api` entitlement; on the OSS image it returns `403` and the operator surfaces a `RequiresEELicense` status condition. A single `LangfuseInstance` remains fully functional on OSS. See [docs/guide/multi-tenancy.md](docs/guide/multi-tenancy.md).

## [0.6.4] - 2026-05-28

### Fixed

- **Migration Job no longer fails when Postgres isn't ready yet.** The Job ran `prisma migrate deploy` immediately; against managed/CNPG Postgres (which take time to start accepting connections) it failed fast and exhausted its backoff limit before the database was up, leaving the Job permanently failed. A `wait-for-stores` init container now blocks until PostgreSQL **and** ClickHouse accept TCP connections (host/port parsed from `DATABASE_URL` / `CLICKHOUSE_URL`) before the migration container runs, with a 5-minute ceiling per store.

## [0.6.3] - 2026-05-22

### Fixed

- **`kubectl delete langfuseinstance` no longer hangs forever.** Seven controllers (instance, secret, migration, retention, schema-drift, circuit-breaker, health-monitor) kept reconciling after the CR was marked for deletion, re-creating the very Deployments that the foreground-deletion GC was trying to remove. Every reconciler now exits early when `metadata.deletionTimestamp` is set, so owner-reference GC can complete and the CR is removed cleanly.
- **Web and worker pods no longer churn (constant ReplicaSet creation).** The instance controller and the secret controller fought over the Deployment's pod-template annotations: the instance controller wrote a Deployment with no `langfuse.palena.ai/secret-hash` annotation, the secret controller patched it back on, the next instance reconcile stripped it again — each flip created a fresh ReplicaSet and terminated the running pods. The instance controller's `reconcileDeployment` now preserves any pod-template annotation under the `langfuse.palena.ai/` namespace from the live Deployment.
- **Helm chart default image tag is now `vX.Y.Z` instead of `X.Y.Z`.** The release workflow has always published images as `vX.Y.Z`, but the chart's helper resolved `.Chart.AppVersion` (no `v`) and produced an unpullable tag, so `helm install` without an explicit `image.tag` 404'd at pull time. The chart's `langfuse-operator.image` helper now defaults to `v<Chart.AppVersion>` (and `app.kubernetes.io/version` follows the same convention); explicit overrides like `--set image.tag=v0.6.2` or `--set image.tag=latest` continue to work verbatim.

## [0.6.2] - 2026-05-22

### Fixed

- **`status.phase` is no longer stuck at `Degraded` on healthy installs.** Previously the four backing-service health checks (`DatabaseReady`, `ClickHouseReady`, `RedisReady`, `BlobStorageReady`) were stubs that always returned `NotConnected`, forcing the phase to `Degraded` even when Langfuse was running fine. The operator now performs real network probes on a 30 s cadence:
  - **PostgreSQL** — TCP dial to the resolved endpoint (CNPG `-rw` service, managed-mode generated secret URL, or external `secretRef`).
  - **ClickHouse** — HTTP GET `/ping` against the managed service URL or external `secretRef` URL.
  - **Redis** — TCP dial + RESP `PING` against the resolved endpoint; accepts `+PONG` *or* `-NOAUTH` as proof the listener is healthy.
  - **Blob storage** — TCP dial against the S3 endpoint (MinIO-style or `s3.<region>.amazonaws.com:443`), Azure Blob (`<account>.blob.core.windows.net:443`), or GCS (`storage.googleapis.com:443`).
  Auth is intentionally *not* tested — port reachability separates "operator can't reach the service" from "auth misconfiguration" (which surfaces in the Langfuse pod logs instead). Each probe has a 3 s timeout.

## [0.6.1] - 2026-05-21

### Fixed

- **Database migration container fails on Langfuse 3.163.0+** — the migration Job hardcoded `node packages/shared/dist/src/db/migrate.cjs`, a path that no longer exists upstream (Langfuse moved Postgres migrations into the image's `entrypoint.sh` via `prisma migrate deploy`). The Job now reuses the image's own ENTRYPOINT and passes `true` as the command, so it picks up whatever migration mechanism the running Langfuse version uses — Postgres and ClickHouse both — and exits cleanly when done. Survives future upstream changes to the migration entrypoint without operator changes.
- **Prisma advisory-lock deadlock during startup** — because the Langfuse image's entrypoint runs `prisma migrate deploy` on every container start, the dedicated migration Job and the web/worker pods raced for the same Postgres advisory lock and deadlocked. The web and worker Deployments now set `LANGFUSE_AUTO_POSTGRES_MIGRATION_DISABLED=true` and `LANGFUSE_AUTO_CLICKHOUSE_MIGRATION_DISABLED=true`, leaving migrations as the sole responsibility of the migration Job.

### Security

- **Bumped `google.golang.org/grpc` to 1.79.3** to resolve [GHSA-p77j-4mvh-x3m3](https://github.com/advisories/GHSA-p77j-4mvh-x3m3) (critical — gRPC-Go authorization bypass via missing leading slash in `:path`).
- **Bumped `go.opentelemetry.io/otel/sdk` to 1.43.0** to resolve [GHSA-9h8m-3fm2-qjrq](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq) and [GHSA-hfvc-g4fc-pqhx](https://github.com/advisories/GHSA-hfvc-g4fc-pqhx) (high — PATH hijacking via OpenTelemetry SDK).
- **Docs site** — overrode `esbuild` to ≥ 0.25.0 and `postcss` to ≥ 8.5.10 to clear two moderate advisories. Vite stays on 5.4.21 (no fix exists for vite 5.x; the remaining advisory is dev-server-only and does not affect the deployed static site).

## [0.6.0] - 2026-04-07

### Added

- **Managed ClickHouse** — deploys a ClickHouse StatefulSet, headless Service, and ConfigMap from `spec.clickhouse.managed` with configurable storage, replicas, resource presets (small/medium/large/custom), and auth secret references
- **Managed Redis** — deploys a Redis StatefulSet and headless Service from `spec.redis.managed` with configurable storage, `requirepass` auth from generated secrets, and persistence via appendonly
- **Database migration controller** — watches for version changes and creates Kubernetes Jobs to run Langfuse database migrations, with status tracking, failure handling, and automatic cleanup of completed jobs
- **Secret generation & rotation** — auto-generates `NEXTAUTH_SECRET`, `SALT`, ClickHouse credentials, and Redis password; detects secret changes via SHA256 hash annotations and triggers rolling restarts
- **ClickHouse retention controller** — manages TTL policies on ClickHouse tables (traces, observations, scores) based on `spec.clickhouse.retention` with configurable per-table TTL days
- **Schema drift detection** — periodic ClickHouse schema validation with configurable check intervals and status condition reporting
- **Circuit breaker** — monitors dependency health (ClickHouse, Redis, PostgreSQL) and scales worker to zero when failure thresholds are breached; auto-restores on recovery
- **Health monitor** — periodic health checks across all components with status condition updates, phase management (Running/Degraded), and event recording on transitions
- **Ingress support** — creates a Kubernetes Ingress from `spec.ingress` with IngressClassName, TLS (manual secret or cert-manager auto-provisioning), and custom annotations
- **OpenShift Route support** — creates an OpenShift Route from `spec.route` with edge TLS termination, optional host, and custom annotations (uses unstructured objects to avoid OpenShift API dependency)
- **Gateway API support** — creates an HTTPRoute from `spec.gatewayAPI` referencing an existing Gateway, with optional hostname and annotations (uses unstructured objects to avoid Gateway API dependency)
- **HorizontalPodAutoscaler** — creates HPAs for Web and Worker deployments from `spec.web.autoscaling` / `spec.worker.autoscaling` with min/max replicas and CPU target utilization
- **PodDisruptionBudget** — creates PDBs for Web and Worker deployments from `spec.web.pdb` / `spec.worker.pdb` with configurable minAvailable
- **ServiceMonitor** — creates a Prometheus ServiceMonitor from `spec.observability.serviceMonitor` (uses unstructured objects to avoid monitoring.coreos.com API dependency)
- **Operator Prometheus metrics** — reconcile count, error count, duration histogram, and managed instance gauge registered with controller-runtime metrics
- **Langfuse Admin API client** — HTTP client with Basic auth for organization, project, member, and API key management via the Langfuse Admin API
- **LangfuseOrganization controller** — full reconciliation with finalizer, member sync (additive and exclusive modes), role-based access, and deletion protection when dependent projects exist
- **LangfuseProject controller** — full reconciliation with finalizer, API key sync, Kubernetes Secret creation with publicKey/secretKey/host, and cascading cleanup on deletion
- **Namespace scoping** — `WATCH_NAMESPACE` env var and `--watch-namespaces` CLI flag to restrict the operator to specific namespaces (comma-separated); defaults to all namespaces. Helm chart exposes `watchNamespaces` value
- **Kind-based E2E test suite** — full-stack E2E tests running in Kind with PostgreSQL, ClickHouse, Redis, and MinIO dependencies; verifies resource creation, labels, owner references, pod health, Langfuse health endpoint, CR updates, garbage collection, and managed data store lifecycle

## [0.5.0] - 2026-04-05

### Added

- **Helm chart** for installing the operator on non-OLM clusters (`deploy/charts/langfuse-operator/`)
- **Automatic CRD sync** into the Helm chart via `make manifests` / `make sync-helm-crds`
- **NetworkPolicy support** — creates per-component policies by default (web: ingress on 3000, worker: deny all ingress; both: egress to data stores and DNS). Disable with `spec.security.networkPolicy.enabled: false`
- **Minikube test manifests** for local end-to-end testing with PostgreSQL, ClickHouse, Redis, and MinIO (`test/minikube/`)

### Fixed

- **ClickHouse migrations fail** — added `CLICKHOUSE_MIGRATION_URL` (native protocol `clickhouse://host:9000`) for both managed and external ClickHouse configurations
- **ClickHouse single-node mode** — set `CLICKHOUSE_CLUSTER_ENABLED=false` by default to prevent `ON CLUSTER` DDL errors without ZooKeeper
- **Web UI unreachable via Service** — set `HOSTNAME=0.0.0.0` on the web container so Next.js binds to all interfaces instead of the pod hostname
- **Lint failures** — extracted phase constants (`goconst`), removed unused error return from `addDatabaseEnv` (`unparam`), reduced `BuildConfig` cyclomatic complexity (`gocyclo`)

## [0.4.0] - 2026-04-02

### Added

- **CRD definitions** for `LangfuseInstance`, `LangfuseOrganization`, and `LangfuseProject` under API group `langfuse.palena.ai/v1alpha1`
- **LangfuseInstance controller** reconciling Web Deployment, Worker Deployment, and Web Service with owner references and status tracking
- **Config generation** computing 50+ environment variables from the CRD spec, covering auth, database (CNPG/managed/external), ClickHouse, Redis, blob storage (S3/Azure/GCS), LLM, telemetry, and OTEL
- **Resource builders** for Web Deployment (HTTP health probes, port 3000, security context), Worker Deployment (exec probe, concurrency config), and ClusterIP Service
- **Full LangfuseInstance spec** with nested types for image, web, worker, auth (email/password, OIDC, init user), secret management (auto-generation, rotation), database, ClickHouse (retention, schema drift, encryption), Redis, blob storage, LLM, ingress, OpenShift Route, security, observability, circuit breaker, and upgrade strategy
- **LangfuseOrganization spec** with member management (additive and exclusive modes) and role-based access
- **LangfuseProject spec** with API key management and Secret creation
- **OLM bundle** with ClusterServiceVersion, RBAC roles, and all three CRDs for Operator Lifecycle Manager deployment
- **Print columns** on all CRDs for `kubectl get` usability
- **Unit tests** for config generation (9 tests), resource builders (10 tests), and controller envtest suite; 96.3% coverage on resources
- **Sample CRs** for minimal instance, production instance, organization, and project
- **VitePress documentation site** with guide pages (installation, quickstart, architecture, database, ClickHouse, Redis, blob storage, auth, networking, observability, upgrades, secrets, multi-tenancy) and CRD reference pages
- **Cloudflare Pages deployment** via `wrangler.toml`
- `CONTRIBUTING.md` with development setup, conventions, and commit format

[Unreleased]: https://github.com/PalenaAI/langfuse-operator/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/PalenaAI/langfuse-operator/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/PalenaAI/langfuse-operator/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/PalenaAI/langfuse-operator/releases/tag/v0.4.0
