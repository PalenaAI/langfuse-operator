# Troubleshooting

When a component won't start, the operator reports the underlying pod failure
directly on the `LangfuseInstance` — you shouldn't need to go hunting through
pods to find out why.

## Start with the CR

```bash
kubectl describe langfuseinstance production -n langfuse
```

The `phase` tells you how to read the situation:

| Phase | Meaning |
|---|---|
| `Pending` | Components are still starting. Normal shortly after create or upgrade. |
| `Migrating` | The migration Job is running. |
| `Running` | All components ready and dependencies reachable. |
| `Degraded` | Something is unhealthy but may still recover on its own (crash loop while a dependency comes up, a failed connectivity probe). |
| `Error` | **Needs you.** A misconfiguration that will never resolve by itself — a bad image reference or a missing Secret key. |

The `Degraded` vs `Error` split is the important one: `Error` means waiting
longer will not help.

## Reading pod issues

`status.web.issues`, `status.worker.issues`, and `status.migration.issues` list
the pod-level problems blocking each component:

```yaml
status:
  phase: Error
  ready: false
  web:
    replicas: 2
    readyReplicas: 0
    issues:
    - pod: production-web-7d9f4c8b6-abc12
      container: langfuse-web
      reason: CreateContainerConfigError
      message: 'secret "production-generated-secrets" key "admin-api-key" not found'
      fatal: true
  conditions:
  - type: WebReady
    status: "False"
    reason: CreateContainerConfigError
    message: 'Web deployment has 0/2 ready replicas; production-web-7d9f4c8b6-abc12
      (langfuse-web): CreateContainerConfigError — secret "production-generated-secrets"
      key "admin-api-key" not found'
```

The same detail lands on the `WebReady` / `WorkerReady` / `MigrationsComplete`
conditions, so `kubectl describe` surfaces it without digging into YAML.

::: tip
Issues are only populated while a component is not ready, and are capped at five
entries — status is a diagnosis, not a log.
:::

## Common reasons

| Reason | Fatal | Typical cause |
|---|---|---|
| `CreateContainerConfigError` | ✅ | A referenced Secret or key doesn't exist. Check the `secretRef` names/keys in your spec, and that the Secret is in the same namespace. |
| `ImagePullBackOff` / `ErrImagePull` | ✅ | Wrong `spec.image.tag`, or a private registry needs `spec.image.pullSecrets`. |
| `InvalidImageName` | ✅ | Malformed repository or tag. |
| `CrashLoopBackOff` | ❌ | The container starts then exits. The message includes the previous run's exit code and output — a Langfuse `ZodError` here means a required env var is missing. Often transient while Postgres/ClickHouse are still starting. |
| `OOMKilled` | ❌ | Raise `spec.web.resources.limits.memory` / `spec.worker.resources.limits.memory`. |
| `Unschedulable` | ❌ | No node satisfies the pod — insufficient CPU/memory, or unsatisfiable `nodeSelector`/`affinity`/`tolerations`. |

## Migrations

A migration that fails or hangs reports through `status.migration` and the
`MigrationsComplete` condition:

```bash
kubectl get langfuseinstance production -n langfuse -o jsonpath='{.status.migration}' | jq
```

The Job's `wait-for-stores` init container blocks until PostgreSQL and
ClickHouse accept TCP connections, so an init container stuck here means the
migration cannot reach a datastore — check the connection Secret and, if you use
`spec.security.networkPolicy`, that egress to the datastore's port is allowed.

## When you still need the pods

The operator surfaces reasons, not logs. For full output:

```bash
kubectl logs -n langfuse -l app.kubernetes.io/instance=production,app.kubernetes.io/component=web
kubectl logs -n langfuse -l app.kubernetes.io/instance=production,app.kubernetes.io/component=worker
kubectl logs -n langfuse -l app.kubernetes.io/instance=production,app.kubernetes.io/component=migration
```

Events on the CR record transitions over time:

```bash
kubectl get events -n langfuse --field-selector involvedObject.name=production
```
