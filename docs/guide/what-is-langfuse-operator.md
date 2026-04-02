# What is Langfuse Operator?

Langfuse Operator is a Kubernetes operator that deploys and manages production-ready [Langfuse](https://langfuse.com) instances. Langfuse is an open-source LLM observability platform for tracing, evaluating, and monitoring AI applications.

## Why an Operator?

Running Langfuse in production involves managing multiple interdependent components:

- **Web** &mdash; the Next.js frontend and API server
- **Worker** &mdash; background job processor for async tasks
- **PostgreSQL** &mdash; primary data store
- **ClickHouse** &mdash; analytics and trace storage
- **Redis** &mdash; caching and job queue
- **Blob Storage** &mdash; S3/Azure/GCS for large trace payloads

Deploying these manually means wiring up dozens of environment variables, managing secrets, coordinating database migrations on upgrades, and monitoring health across all components. The operator handles all of this declaratively.

## What It Manages

With a single `LangfuseInstance` custom resource, the operator:

- Creates and configures Web and Worker Deployments
- Wires up all component connectivity via environment variables
- Manages database migrations and background migrations on upgrades
- Generates and rotates secrets automatically
- Creates Services, Ingress/Route, NetworkPolicies, HPAs, and PDBs
- Monitors component health with circuit breakers
- Tracks ClickHouse storage pressure and schema drift

Additional CRDs enable multi-tenancy:

- **LangfuseOrganization** &mdash; manages Langfuse organizations via the Admin API
- **LangfuseProject** &mdash; manages projects and API keys, storing credentials in Kubernetes Secrets

## Custom Resources

| Kind | Purpose |
|---|---|
| `LangfuseInstance` | Deploys the complete Langfuse stack |
| `LangfuseOrganization` | Manages a Langfuse organization |
| `LangfuseProject` | Manages a project and its API keys |

All CRDs use the API group `langfuse.palena.ai/v1alpha1`.

## Platform Support

| Platform | Notes |
|---|---|
| Kubernetes 1.26+ | Standard deployment |
| OpenShift 4.12+ | Route support, SCC-compatible |
| Amazon EKS | S3 native, IRSA/Pod Identity |
| Google GKE | GCS native, Workload Identity |
| Azure AKS | Azure Blob Storage, Azure AD OIDC |
| Rancher / RKE2 | Standard deployment |

## Next Steps

- [Architecture](/guide/architecture) &mdash; understand the operator's internal design
- [Installation](/guide/installation) &mdash; install the operator in your cluster
- [Quick Start](/guide/quickstart) &mdash; deploy your first Langfuse instance
