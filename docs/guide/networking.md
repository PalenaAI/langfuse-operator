# Networking

## Ingress

Expose Langfuse via a Kubernetes Ingress:

```yaml
spec:
  ingress:
    enabled: true
    className: nginx
    host: langfuse.example.com
    annotations:
      nginx.ingress.kubernetes.io/proxy-body-size: "50m"
    tls:
      enabled: true
      secretName: langfuse-tls          # existing TLS secret
```

### With cert-manager

Automatically provision TLS certificates:

```yaml
spec:
  ingress:
    enabled: true
    className: nginx
    host: langfuse.example.com
    tls:
      enabled: true
      certManager:
        issuerRef:
          name: letsencrypt-prod
          kind: ClusterIssuer
```

## OpenShift Route

On OpenShift, use a Route instead of Ingress:

```yaml
spec:
  route:
    enabled: true
    host: langfuse.apps.example.com
    annotations: {}
```

::: tip
Only one of `ingress` or `route` should be enabled.
:::

## NetworkPolicy

The operator creates per-component NetworkPolicies by default that restrict traffic to only what Langfuse needs:

```yaml
spec:
  security:
    networkPolicy:
      enabled: true    # default: true
```

To disable:

```yaml
spec:
  security:
    networkPolicy:
      enabled: false
```

### Web NetworkPolicy (`<name>-web-netpol`)

| Direction | Rule |
|-----------|------|
| **Ingress** | Allow TCP port 3000 from any source |
| **Egress** | Allow TCP to PostgreSQL (5432), ClickHouse (8123, 9000), Redis (6379), HTTPS (443), internal (3000) |
| **Egress** | Allow DNS (UDP+TCP port 53) |

### Worker NetworkPolicy (`<name>-worker-netpol`)

| Direction | Rule |
|-----------|------|
| **Ingress** | Deny all (worker exposes no ports) |
| **Egress** | Same as Web |

Both policies are owned by the `LangfuseInstance` CR and are automatically deleted when the instance is removed.
