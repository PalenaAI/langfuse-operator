# Multi-Tenancy

The operator provides two additional CRDs for managing Langfuse organizations and projects declaratively.

::: danger Requires a Langfuse Enterprise/Pro self-hosted license
`LangfuseOrganization` and `LangfuseProject` are powered by Langfuse's [organization-management API](https://langfuse.com/self-hosting/organization-management-api), which is an **Enterprise/Pro self-hosted feature**. On the **OSS** `langfuse/langfuse` image this API returns `403 "This feature is not available on your current plan."`, so these CRDs **do not work without a license**.

To use them you must:

1. Hold a Langfuse **self-hosted Pro or Enterprise** license key (`langfuse_ee_...`).
2. Provide it via `spec.eeLicenseKey` on the `LangfuseInstance` (see below). The operator injects it as `LANGFUSE_EE_LICENSE_KEY` and the Langfuse server then enables the admin API.

If the license is missing, the operator does not fail the instance — it surfaces a `RequiresEELicense` condition on the affected `LangfuseOrganization`/`LangfuseProject` and leaves a single `LangfuseInstance` fully functional.
:::

## Enabling: EE license key

```yaml
apiVersion: langfuse.palena.ai/v1alpha1
kind: LangfuseInstance
metadata:
  name: production
  namespace: langfuse
spec:
  eeLicenseKey:
    secretRef:
      name: langfuse-ee-license
      key: license-key
  # ... rest of the instance spec
```

Create the Secret first:

```bash
kubectl create secret generic langfuse-ee-license \
  -n langfuse --from-literal=license-key="langfuse_ee_..."
```

After the instance pods restart with `LANGFUSE_EE_LICENSE_KEY` set, the Organization/Project CRDs reconcile normally.

## Prerequisite: ADMIN_API_KEY

The `LangfuseOrganization` and `LangfuseProject` controllers manage resources through the Langfuse [organization-management API](https://langfuse.com/self-hosting/organization-management-api), which authenticates with the instance's `ADMIN_API_KEY` as a Bearer token.

The operator handles this for you:

- **Auto-generated (default):** if `spec.auth.adminApiKey` is omitted, the operator generates an `ADMIN_API_KEY`, stores it in the instance's auto-generated secret (`<instance>-generated-secrets`, key `admin-api-key`), injects it into the Langfuse Web/Worker containers, and uses it for org/project reconciliation. No action required.
- **User-provided:** to supply your own key, set `spec.auth.adminApiKey.secretRef`:

  ```yaml
  apiVersion: langfuse.palena.ai/v1alpha1
  kind: LangfuseInstance
  metadata:
    name: production
    namespace: langfuse
  spec:
    auth:
      adminApiKey:
        secretRef:
          name: langfuse-admin
          key: admin-api-key
  ```

  The same key is injected into the Langfuse containers and used by the operator, so the two always match.

::: warning
`LangfuseOrganization` / `LangfuseProject` CRs reconcile only after the referenced `LangfuseInstance` is running with `ADMIN_API_KEY` configured. Creating them against an instance that predates this setting requires restarting the instance pods so the new env var takes effect.
:::

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

### Wiring clients to a `LangfuseProject`

The Secret created by the operator (`spec.apiKeys[].secretName`) contains three keys that match the env-var convention used by virtually every Langfuse client SDK:

| Secret key | Typical client env var |
|---|---|
| `publicKey` | `LANGFUSE_PUBLIC_KEY` |
| `secretKey` | `LANGFUSE_SECRET_KEY` |
| `host` | `LANGFUSE_BASE_URL` (or `LANGFUSE_HOST`) |

So any pod that wants to send traces to Langfuse just mounts those keys as env vars — no glue code required.

#### Example: LibreChat

LibreChat ([reads](https://www.librechat.ai/docs/configuration/langfuse) `LANGFUSE_PUBLIC_KEY` / `LANGFUSE_SECRET_KEY` / `LANGFUSE_BASE_URL`):

```yaml
apiVersion: langfuse.palena.ai/v1alpha1
kind: LangfuseProject
metadata:
  name: librechat
  namespace: librechat
spec:
  instanceRef:
    name: production
    namespace: langfuse
  organizationRef:
    name: ml-platform
  projectName: "LibreChat"
  apiKeys:
    - name: librechat
      secretName: librechat-langfuse-keys
```

Then in your LibreChat Deployment:

```yaml
containers:
  - name: librechat
    env:
      - name: LANGFUSE_PUBLIC_KEY
        valueFrom: { secretKeyRef: { name: librechat-langfuse-keys, key: publicKey } }
      - name: LANGFUSE_SECRET_KEY
        valueFrom: { secretKeyRef: { name: librechat-langfuse-keys, key: secretKey } }
      - name: LANGFUSE_BASE_URL
        valueFrom: { secretKeyRef: { name: librechat-langfuse-keys, key: host } }
```

The same pattern wires any other client (Helicone, Promptfoo, custom Node/Python apps using `langfuse` / `langfuse-python` SDKs) — only the destination env-var names differ.

::: tip Cross-cluster clients
The `host` value the operator writes is the in-cluster service URL (`http://<instance>-web.<ns>.svc:3000`). If the client lives in a different cluster or outside the cluster entirely, override `LANGFUSE_BASE_URL` to your external Ingress/Route URL instead of using the Secret's `host` key.
:::

### Project Deletion

The finalizer `langfuse.palena.ai/project-cleanup`:

1. Revokes all API keys via the Admin API
2. Deletes the associated Kubernetes Secrets
3. Optionally deletes the project in Langfuse (controlled by annotation `langfuse.palena.ai/delete-on-remove: "true"`, default: keep)
