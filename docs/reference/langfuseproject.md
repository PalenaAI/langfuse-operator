# LangfuseProject

`langfuseprojects.langfuse.palena.ai/v1alpha1`

Manages a Langfuse project and its API keys via the Admin API.

## Spec

| Field | Type | Description |
|---|---|---|
| `instanceRef` | ObjectReference | Reference to the parent `LangfuseInstance` |
| `organizationRef` | ObjectReference | Reference to the parent `LangfuseOrganization` CR |
| `projectName` | string | Project name in Langfuse |
| `apiKeys` | []APIKeySpec | API keys to create and manage |

### APIKeySpec

| Field | Type | Description |
|---|---|---|
| `name` | string | Logical name of the API key |
| `secretName` | string | Kubernetes Secret name to store the key pair |

## Status

| Field | Type | Description |
|---|---|---|
| `ready` | bool | Whether the project is fully synced |
| `projectId` | string | Langfuse internal project ID |
| `organizationId` | string | Langfuse internal organization ID |
| `apiKeys` | []APIKeyStatus | Status of each managed API key |
| `conditions` | []Condition | Standard Kubernetes conditions |

### APIKeyStatus

| Field | Type | Description |
|---|---|---|
| `name` | string | Logical name |
| `secretName` | string | Kubernetes Secret name |
| `created` | bool | Whether the key has been created |
| `lastRotated` | *Time | Last rotation timestamp |

### Conditions

| Type | Description |
|---|---|
| `Ready` | Project exists and is synced |
| `Synced` | Spec matches Langfuse state |
| `APIKeysReady` | All API keys are created and valid |

## Finalizer

`langfuse.palena.ai/project-cleanup`

On deletion:
1. Revokes all API keys via the Admin API
2. Deletes associated Kubernetes Secrets
3. Optionally deletes the project (annotation `langfuse.palena.ai/delete-on-remove: "true"`)
4. Removes the finalizer

## API Key Secret Format

Each API key entry creates a Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: langfuse-ml-team-keys
  labels:
    app.kubernetes.io/managed-by: langfuse-operator
    langfuse.palena.ai/instance: production
    langfuse.palena.ai/project: ml-team-prod
type: Opaque
data:
  publicKey: <base64>      # pk-lf-...
  secretKey: <base64>      # sk-lf-...
  host: <base64>           # https://langfuse.example.com
```

## Example

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

### Print Columns

```
NAME           READY   PROJECT ID     AGE
ml-team-prod   true    proj_xyz789    2d
```
