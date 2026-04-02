# Multi-Tenancy

The operator provides two additional CRDs for managing Langfuse organizations and projects declaratively.

## Organizations

A `LangfuseOrganization` maps to an organization in Langfuse:

```yaml
apiVersion: langfuse.palena.ai/v1alpha1
kind: LangfuseOrganization
metadata:
  name: ml-platform
  namespace: langfuse
spec:
  instanceRef:
    name: production
  displayName: "ML Platform Team"
  members:
    managedExclusively: false
    users:
      - email: "alice@example.com"
        role: owner
      - email: "bob@example.com"
        role: admin
      - email: "carol@example.com"
        role: member
```

### Member Management

- `managedExclusively: false` (default) &mdash; the operator adds users from the list but never removes users added outside the CR
- `managedExclusively: true` &mdash; the operator enforces the list exactly, removing users not present

Available roles: `owner`, `admin`, `member`, `viewer`.

### Deletion

The operator uses a finalizer (`langfuse.palena.ai/organization-cleanup`). On deletion:

1. Checks for `LangfuseProject` CRs referencing this organization
2. If projects exist, blocks deletion and sets a `DeletionBlocked` condition
3. If no projects reference it, deletes the organization via the Langfuse Admin API
4. Removes the finalizer

## Projects

A `LangfuseProject` maps to a project within an organization:

```yaml
apiVersion: langfuse.palena.ai/v1alpha1
kind: LangfuseProject
metadata:
  name: ml-team-prod
  namespace: langfuse
spec:
  instanceRef:
    name: production
  organizationRef:
    name: ml-platform
  projectName: "prod-inference"
  apiKeys:
    - name: default
      secretName: langfuse-ml-team-keys
    - name: ci-pipeline
      secretName: langfuse-ci-keys
```

### API Key Management

For each entry in `spec.apiKeys`, the operator:

1. Creates an API key in Langfuse via the Admin API
2. Stores the key pair in a Kubernetes Secret

The created Secret has this format:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: langfuse-ml-team-keys
type: Opaque
data:
  publicKey: <base64>      # pk-lf-...
  secretKey: <base64>      # sk-lf-...
  host: <base64>           # https://langfuse.example.com
```

Workloads can mount these directly:

```yaml
env:
  - name: LANGFUSE_PUBLIC_KEY
    valueFrom:
      secretKeyRef:
        name: langfuse-ml-team-keys
        key: publicKey
  - name: LANGFUSE_SECRET_KEY
    valueFrom:
      secretKeyRef:
        name: langfuse-ml-team-keys
        key: secretKey
  - name: LANGFUSE_HOST
    valueFrom:
      secretKeyRef:
        name: langfuse-ml-team-keys
        key: host
```

### Project Deletion

The finalizer `langfuse.palena.ai/project-cleanup`:

1. Revokes all API keys via the Admin API
2. Deletes the associated Kubernetes Secrets
3. Optionally deletes the project in Langfuse (controlled by annotation `langfuse.palena.ai/delete-on-remove: "true"`, default: keep)
