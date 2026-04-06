<p align="center">
  <img src="docs/public/logo.svg" alt="Langfuse" width="120" />
</p>

<h1 align="center">Langfuse Operator</h1>

<p align="center">
  A Kubernetes operator for deploying and managing production-ready
  <a href="https://langfuse.com">Langfuse</a> LLM observability instances.
</p>

<p align="center">
  <a href="https://github.com/PalenaAI/langfuse-operator/actions"><img src="https://github.com/PalenaAI/langfuse-operator/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/PalenaAI/langfuse-operator/releases"><img src="https://img.shields.io/github/v/release/PalenaAI/langfuse-operator" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/PalenaAI/langfuse-operator" alt="License"></a>
</p>

---

## Overview

The Langfuse Operator manages the full [Langfuse v3](https://langfuse.com) stack on Kubernetes through a declarative API. Define a `LangfuseInstance` custom resource and the operator handles deployments, services, configuration, and lifecycle management.

### Key Features

- **Full stack deployment** -- Web, Worker, PostgreSQL, ClickHouse, Redis, and Blob Storage from a single CR
- **Automated upgrades** -- zero-downtime rollouts with database migration orchestration
- **Secret management** -- auto-generation and rotation with rolling restarts
- **Networking** -- Kubernetes Ingress (with TLS/cert-manager), OpenShift Routes, Gateway API HTTPRoute, and per-component NetworkPolicies
- **Multi-tenancy** -- manage organizations, projects, and API keys via `LangfuseOrganization` and `LangfuseProject` CRDs
- **Observability** -- Prometheus ServiceMonitor, OpenTelemetry integration, and operator metrics
- **Platform support** -- Kubernetes, OpenShift, EKS, GKE, and AKS

## Quick Start

### Prerequisites

- Kubernetes v1.26+
- kubectl
- [OLM](https://olm.operatorframework.io/) (for OLM-based install) or Helm v3

### Install with Helm

```bash
helm install langfuse-operator deploy/charts/langfuse-operator \
  -n langfuse-operator-system --create-namespace \
  --set image.tag=0.5.0
```

This installs the CRDs, RBAC, and operator deployment. See the [chart values](deploy/charts/langfuse-operator/values.yaml) for all configuration options.

### Install CRDs only (manual deploy)

```bash
kubectl apply -f https://raw.githubusercontent.com/PalenaAI/langfuse-operator/main/config/crd/bases/langfuse.palena.ai_langfuseinstances.yaml
kubectl apply -f https://raw.githubusercontent.com/PalenaAI/langfuse-operator/main/config/crd/bases/langfuse.palena.ai_langfuseorganizations.yaml
kubectl apply -f https://raw.githubusercontent.com/PalenaAI/langfuse-operator/main/config/crd/bases/langfuse.palena.ai_langfuseprojects.yaml
```

### Deploy a Langfuse Instance

```yaml
apiVersion: langfuse.palena.ai/v1alpha1
kind: LangfuseInstance
metadata:
  name: langfuse
  namespace: langfuse
spec:
  image:
    tag: "3"
  auth:
    nextAuthUrl: "https://langfuse.example.com"
```

```bash
kubectl apply -f langfuse-instance.yaml
```

The operator creates and manages the Web and Worker deployments, Services, managed data stores (ClickHouse, Redis), database migrations, secrets, networking, and observability resources.

### Verify

```bash
kubectl get langfuseinstances -n langfuse
```

```
NAME       PHASE     READY   VERSION   AGE
langfuse   Running   true    3         2m
```

## Custom Resources

| CRD | Purpose |
|-----|---------|
| `LangfuseInstance` | Deploys and manages a full Langfuse stack |
| `LangfuseOrganization` | Manages organizations and member access |
| `LangfuseProject` | Manages projects and API key Secrets |

## Documentation

Full documentation is available at the [Langfuse Operator Docs](https://langfuse-operator.pages.dev) site, covering:

- [Installation](https://langfuse-operator.pages.dev/guide/installation) -- OLM, Helm, and manual methods
- [Architecture](https://langfuse-operator.pages.dev/guide/architecture) -- how the operator works
- [Database](https://langfuse-operator.pages.dev/guide/database), [ClickHouse](https://langfuse-operator.pages.dev/guide/clickhouse), [Redis](https://langfuse-operator.pages.dev/guide/redis), [Blob Storage](https://langfuse-operator.pages.dev/guide/blob-storage) -- data store configuration
- [Authentication](https://langfuse-operator.pages.dev/guide/authentication) -- OIDC, email/password, init user
- [Networking](https://langfuse-operator.pages.dev/guide/networking) -- Ingress, OpenShift Routes, NetworkPolicies
- [Upgrades](https://langfuse-operator.pages.dev/guide/upgrades) -- zero-downtime upgrade strategy
- [Secret Management](https://langfuse-operator.pages.dev/guide/secrets) -- auto-generation and rotation
- [Multi-Tenancy](https://langfuse-operator.pages.dev/guide/multi-tenancy) -- organizations and projects
- [CRD Reference](https://langfuse-operator.pages.dev/reference/langfuseinstance) -- full spec documentation

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding conventions, and how to submit changes.

## License

Copyright 2026 bitkaio LLC. Licensed under the [Apache License 2.0](LICENSE).
