# Changelog

All notable changes to the Langfuse Operator will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Health monitor probes are now real.** Previously the four backing-service health checks (`DatabaseReady`, `ClickHouseReady`, `RedisReady`, `BlobStorageReady`) were stubs that always returned `NotConnected`, forcing `status.phase` to `Degraded` even when the install was healthy. The operator now performs actual network probes on a 30 s cadence:
  - **PostgreSQL** — TCP dial to the resolved endpoint (CNPG `-rw` service, managed-mode generated secret URL, or external `secretRef`).
  - **ClickHouse** — HTTP GET `/ping` against the managed service URL or external `secretRef` URL.
  - **Redis** — TCP dial + RESP `PING` against the resolved endpoint; accepts `+PONG` *or* `-NOAUTH` as proof the listener is healthy.
  - **Blob storage** — TCP dial against the S3 endpoint (MinIO style or `s3.<region>.amazonaws.com:443`), Azure Blob (`<account>.blob.core.windows.net:443`), or GCS (`storage.googleapis.com:443`). Auth is intentionally *not* tested — port reachability separates "operator can't reach the service" from "auth misconfiguration" (which surfaces in the Langfuse pod logs instead). Each probe has a 3 s timeout.

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
