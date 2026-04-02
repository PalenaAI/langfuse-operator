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

The operator creates a NetworkPolicy by default that restricts traffic to only what Langfuse needs:

```yaml
spec:
  security:
    networkPolicy:
      enabled: true    # default: true
```

The generated policy allows:

- Ingress to Web pods on port 3000 (from Ingress controller or any pod)
- Egress to PostgreSQL, ClickHouse, Redis, and blob storage endpoints
- Egress for DNS resolution
