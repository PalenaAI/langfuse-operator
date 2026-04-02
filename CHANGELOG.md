# Changelog

All notable changes to the Langfuse Operator will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **CRD definitions** for `LangfuseInstance`, `LangfuseOrganization`, and `LangfuseProject` under API group `langfuse.palena.ai/v1alpha1`
- **LangfuseInstance controller** reconciling Web Deployment, Worker Deployment, and Web Service with owner references and status tracking
- **Config generation** (`internal/langfuse/config.go`) computing 50+ environment variables from the CRD spec, covering auth, database (CNPG/managed/external), ClickHouse, Redis, blob storage (S3/Azure/GCS), LLM, telemetry, and OTEL
- **Resource builders** for Web Deployment (HTTP health probes, port 3000, security context), Worker Deployment (exec probe, concurrency config), and ClusterIP Service
- **Full LangfuseInstance spec** with nested types for image, web, worker, auth (email/password, OIDC, init user), secret management (auto-generation, rotation), database, ClickHouse (retention, schema drift, encryption), Redis, blob storage, LLM, ingress, OpenShift Route, security, observability, circuit breaker, and upgrade strategy
- **LangfuseOrganization spec** with member management (additive and exclusive modes) and role-based access
- **LangfuseProject spec** with API key management and Secret creation
- **Print columns** on all CRDs for `kubectl get` usability
- **Unit tests** for config generation (9 tests), resource builders (10 tests), and controller envtest suite; 96.3% coverage on resources
- **Sample CRs** for minimal instance, production instance, organization, and project
- **VitePress documentation site** (`docs/`) with guide pages (installation, quickstart, architecture, database, ClickHouse, Redis, blob storage, auth, networking, observability, upgrades, secrets, multi-tenancy) and CRD reference pages
- **Cloudflare Pages deployment** via `wrangler.toml`
- `CONTRIBUTING.md` with development setup, conventions, and commit format

[Unreleased]: https://github.com/PalenaAI/langfuse-operator/commits/main
