# Contributing to Langfuse Operator

Thank you for your interest in contributing to the Langfuse Operator! This guide will help you get started.

## Prerequisites

- Go 1.22+
- Docker 17.03+
- kubectl v1.26+
- Access to a Kubernetes v1.26+ cluster (or [kind](https://kind.sigs.k8s.io/) for local development)
- [Operator SDK](https://sdk.operatorframework.io/) v1.38+

## Development Setup

1. **Clone the repository:**

   ```bash
   git clone https://github.com/bitkaio/langfuse-operator.git
   cd langfuse-operator
   ```

2. **Install dependencies:**

   ```bash
   go mod download
   ```

3. **Verify the build:**

   ```bash
   make manifests generate build
   ```

## Common Tasks

### Building

```bash
make build                  # compile the operator binary
make manifests              # regenerate CRD manifests from Go types
make generate               # regenerate deepcopy methods
make docker-build IMG=...   # build the container image
```

### Testing

```bash
make test                   # run unit + envtest tests (no cluster needed)
make test-e2e               # run e2e tests (requires kind cluster)
make lint                   # run golangci-lint
make vet                    # run go vet
```

Run a single test:

```bash
go test ./internal/controller/... -run TestName -v
```

### Running Locally

```bash
kind create cluster          # create a local cluster
make install                 # install CRDs into the cluster
make run                     # run the operator against current kubeconfig
```

Apply a sample CR:

```bash
kubectl apply -f config/samples/langfuse_v1alpha1_langfuseinstance.yaml
```

## Project Structure

```
api/v1alpha1/               CRD type definitions
internal/controller/         Reconciler implementations
internal/langfuse/           Langfuse config generation and API client
internal/resources/          Kubernetes resource builders (one file per kind)
config/crd/bases/            Generated CRD YAML
config/rbac/                 RBAC manifests
config/samples/              Example custom resources
test/                        Integration and e2e tests
```

## Code Conventions

- **Error wrapping:** always `fmt.Errorf("context: %w", err)` — never bare `err` returns.
- **No `panic`** in controller code. Return errors and let the reconciler requeue.
- **Table-driven tests** preferred. Use `envtest` for integration tests.
- **Kubebuilder markers** (`//+kubebuilder:...`) for RBAC, validation, and defaulting. Never hand-edit generated files (`zz_generated.deepcopy.go`, CRD YAML).
- **Resource builders** go in `internal/resources/`, one file per Kubernetes resource kind. Controllers call builders and then `CreateOrUpdate`.
- **Labels** on all managed resources follow the [standard Kubernetes labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/):
  - `app.kubernetes.io/name: langfuse`
  - `app.kubernetes.io/instance: <instance-name>`
  - `app.kubernetes.io/component: web|worker|...`
  - `app.kubernetes.io/managed-by: langfuse-operator`

## Making Changes

1. **Create a branch** from `main` for your work.
2. **Write or update types** in `api/v1alpha1/` if changing the CRD schema.
3. **Run `make manifests generate`** after modifying types to regenerate CRDs and deepcopy.
4. **Add or update tests** for any new or changed logic.
5. **Run `make test lint`** and fix any failures before submitting.
6. **Open a pull request** with a clear description of what changed and why.

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(controller): add health monitoring for ClickHouse
fix(config): correct Redis TLS env var mapping
docs(readme): update prerequisites section
test(resources): add coverage for PDB builder
```

## Reporting Issues

Open an issue on GitHub with:

- A clear description of the problem or feature request
- Steps to reproduce (for bugs)
- Operator version, Kubernetes version, and platform (EKS, GKE, OpenShift, etc.)

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
