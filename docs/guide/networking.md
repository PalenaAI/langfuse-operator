# Networking

The operator manages Ingress, OpenShift Route, Gateway API HTTPRoute, and NetworkPolicy resources automatically based on your `LangfuseInstance` spec. All created resources are owned by the CR and cleaned up on deletion.

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

When using cert-manager, the operator automatically:

- Adds the `cert-manager.io/cluster-issuer` or `cert-manager.io/issuer` annotation based on the issuer kind
- Generates a TLS secret name (`<instance>-web-tls`) if none is specified

## OpenShift Route

On OpenShift, use a Route instead of Ingress. The operator creates the Route as an unstructured object, so no OpenShift API dependency is required:

```yaml
spec:
  route:
    enabled: true
    host: langfuse.apps.example.com
    annotations: {}
```

::: tip
Only one of `ingress`, `route`, or `gatewayAPI` should be enabled.
:::

## Gateway API

If your cluster uses the [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/), create an HTTPRoute that attaches to an existing Gateway:

```yaml
spec:
  gatewayAPI:
    enabled: true
    gatewayRef:
      name: my-gateway
      namespace: gateway-system   # optional, defaults to CR namespace
      sectionName: https          # optional, target a specific listener
    hostname: langfuse.example.com
    annotations: {}
```

The operator creates the HTTPRoute as an unstructured object, so the Gateway API CRDs do not need to be a Go dependency. The Gateway itself must be provisioned separately (by the platform team or a Gateway controller like Envoy Gateway, Istio, or Cilium).

::: tip
The HTTPRoute routes all traffic (`PathPrefix: /`) to the Web Service on port 3000.
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
