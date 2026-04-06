# Changelog

All notable changes to the Langfuse Operator will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Ingress support** — creates a Kubernetes Ingress from `spec.ingress` with IngressClassName, TLS (manual secret or cert-manager auto-provisioning), and custom annotations
- **OpenShift Route support** — creates an OpenShift Route from `spec.route` with edge TLS termination, optional host, and custom annotations (uses unstructured objects to avoid OpenShift API dependency)
- **Gateway API support** — creates an HTTPRoute from `spec.gatewayAPI` referencing an existing Gateway, with optional hostname and annotations (uses unstructured objects to avoid Gateway API dependency)

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

[Unreleased]: https://github.com/PalenaAI/langfuse-operator/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/PalenaAI/langfuse-operator/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/PalenaAI/langfuse-operator/releases/tag/v0.4.0
