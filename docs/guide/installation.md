# Installation

## Prerequisites

- Kubernetes 1.26+ cluster
- `kubectl` configured for your cluster
- Cluster-admin privileges (for CRD installation)

## Install with OLM

If your cluster has the [Operator Lifecycle Manager](https://olm.operatorframework.io/) installed (e.g., OpenShift):

```bash
# Create a CatalogSource
kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: langfuse-operator-catalog
  namespace: olm
spec:
  sourceType: grpc
  image: ghcr.io/PalenaAI/langfuse-operator-catalog:latest
  displayName: Langfuse Operator
EOF

# Create a Subscription
kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: langfuse-operator
  namespace: operators
spec:
  channel: stable
  name: langfuse-operator
  source: langfuse-operator-catalog
  sourceNamespace: olm
EOF
```

## Install with Helm

For clusters without OLM:

```bash
helm install langfuse-operator deploy/charts/langfuse-operator \
  --namespace langfuse-operator-system \
  --create-namespace \
  --set image.tag=0.5.0
```

See the [chart values](https://github.com/PalenaAI/langfuse-operator/blob/main/deploy/charts/langfuse-operator/values.yaml) for all configuration options (replicas, resources, tolerations, affinity, etc.).

## Install with Manifests

Apply the raw manifests directly:

```bash
kubectl apply -f https://raw.githubusercontent.com/PalenaAI/langfuse-operator/main/dist/install.yaml
```

Or build from source:

```bash
git clone https://github.com/PalenaAI/langfuse-operator.git
cd langfuse-operator
make install   # Install CRDs
make deploy    # Deploy the operator
```

## Verify Installation

```bash
# Check the operator pod is running
kubectl get pods -n langfuse-operator-system

# Check CRDs are installed
kubectl get crds | grep langfuse
```

Expected CRDs:

```
langfuseinstances.langfuse.palena.ai
langfuseorganizations.langfuse.palena.ai
langfuseprojects.langfuse.palena.ai
```

## Uninstall

```bash
# Remove all Langfuse CRs first (this triggers cleanup)
kubectl delete langfuseinstances --all -A
kubectl delete langfuseprojects --all -A
kubectl delete langfuseorganizations --all -A

# Then remove the operator
make undeploy   # or helm uninstall / delete OLM subscription
make uninstall  # remove CRDs
```

::: warning
Deleting CRDs will remove **all** Langfuse custom resources and their owned objects (Deployments, Services, Secrets, etc.). Always delete CRs before CRDs to ensure clean finalization.
:::
