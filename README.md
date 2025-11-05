[![CircleCI](https://circleci.com/gh/giantswarm/silence-operator.svg?&style=shield)](https://circleci.com/gh/giantswarm/silence-operator)
[![Docker Repository on Quay](https://quay.io/repository/giantswarm/silence-operator/status "Docker Repository on Quay")](https://quay.io/repository/giantswarm/silence-operator)

# silence-operator

The Silence Operator automates the management of [Alertmanager](https://github.com/prometheus/alertmanager) [silences](https://prometheus.io/docs/alerting/latest/alertmanager/#silences) using Kubernetes Custom Resources. This allows you to define and manage silences declaratively, just like other Kubernetes objects, integrating them into your GitOps workflows.

## Prerequisites

* Kubernetes 1.25+
* Helm 3+

---

## Install

You can now install the chart using either of the following methods:

### Method 1: From OCI Registry (Recommended)

```console
helm install [RELEASE_NAME] oci://gsoci.azurecr.io/charts/giantswarm/silence-operator --version [VERSION]
```

To upgrade an existing release:

```console
helm upgrade [RELEASE_NAME] oci://gsoci.azurecr.io/charts/giantswarm/silence-operator --version [VERSION]
```

### Method 2: From Helm Repository (Legacy)

```console
helm repo add giantswarm https://giantswarm.github.io/control-plane-catalog/
helm repo update
```

*See [`helm repo`](https://helm.sh/docs/helm/helm_repo/) for command documentation.*

```console
helm install [RELEASE_NAME] giantswarm/silence-operator
```

You can customize the installation by providing your own values file or by overriding values on the command line. For example:

```console
helm install [RELEASE_NAME] giantswarm/silence-operator \
  --set alertmanagerAddress="http://my-alertmanager:9093" \
  --set silenceSelector="environment=production" \
  --set namespaceSelector="team=platform"
```

*See [helm install](https://helm.sh/docs/helm/helm_install/) for command documentation.*

---

## CRDs

CRDs are **not created automatically** by this chart and should be manually deployed:

```console
# For existing cluster-scoped silences (legacy)
kubectl apply --server-side -f https://raw.githubusercontent.com/giantswarm/silence-operator/main/config/crd/monitoring.giantswarm.io_silences.yaml

# For new namespace-scoped silences (recommended)
kubectl apply --server-side -f https://raw.githubusercontent.com/giantswarm/silence-operator/main/config/crd/observability.giantswarm.io_silences.yaml
```

**Note**: The operator supports both API versions for backward compatibility. New deployments should use the namespace-scoped `observability.giantswarm.io/v1alpha2` API. See [MIGRATION.md](MIGRATION.md) for migration guidance.

---

## Uninstall Helm Chart

```console
helm uninstall [RELEASE_NAME]
```

This removes all the Kubernetes components associated with the chart and deletes the release.

*See [helm uninstall](https://helm.sh/docs/helm/helm_uninstall/) for command documentation.*

CRDs are not removed by default and should be manually cleaned up:

```console
# Remove legacy cluster-scoped CRD
kubectl delete crd silences.monitoring.giantswarm.io

# Remove namespace-scoped CRD
kubectl delete crd silences.observability.giantswarm.io
```

---

## Upgrading Chart

### OCI Registry Upgrade (Recommended)

```console
helm upgrade [RELEASE_NAME] oci://gsoci.azurecr.io/charts/giantswarm/silence-operator --version [VERSION]
```

### Helm Repository Upgrade (Legacy)

```console
helm upgrade [RELEASE_NAME] giantswarm/silence-operator
```

CRDs should be manually updated after upgrading:

```console
# Update legacy cluster-scoped CRD (if using v1alpha1)
kubectl apply --server-side -f https://raw.githubusercontent.com/giantswarm/silence-operator/main/config/crd/monitoring.giantswarm.io_silences.yaml

# Update namespace-scoped CRD (if using v1alpha2)
kubectl apply --server-side -f https://raw.githubusercontent.com/giantswarm/silence-operator/main/config/crd/observability.giantswarm.io_silences.yaml
```

*See [helm upgrade](https://helm.sh/docs/helm/helm_upgrade/) for command documentation.*

---

## Configuration

The silence-operator can be configured through Helm values to control which resources it processes.

### Silence Selector

Filter which `Silence` custom resources the operator processes based on their labels. This applies to both v1alpha1 and v1alpha2 APIs.

```yaml
# values.yaml
silenceSelector: "environment=production,tier=frontend"
```

**Examples:**

* `"environment=production"` - Only process silences with `environment=production` label
* `"team=platform,tier=monitoring"` - Only process silences with both labels
* `"environment in (production,staging)"` - Process silences with environment in the specified set
* `""` - Process all silence resources (default)

### Namespace Selector

Restrict which namespaces the v2 controller watches for `Silence` CRs. This only applies to the namespace-scoped `observability.giantswarm.io/v1alpha2` API.

```yaml
# values.yaml
namespaceSelector: "environment=production"
```

**Examples:**

* `"environment=production"` - Only watch namespaces labeled with `environment=production`
* `"team=platform,tier=monitoring"` - Only watch namespaces with both labels
* `"team notin (test,staging)"` - Watch namespaces except those with specified team labels
* `""` - Watch all namespaces (default)

---

## Overview

### API Versions and Migration

The silence-operator supports two API versions:

* **`monitoring.giantswarm.io/v1alpha1`** - Legacy cluster-scoped API (deprecated)
* **`observability.giantswarm.io/v1alpha2`** - New namespace-scoped API (recommended)

**Migration Notice**: The v1alpha1 API is deprecated but remains fully supported for backward compatibility. New deployments should use the v1alpha2 API for better multi-tenancy and namespace isolation. Existing v1alpha1 silences continue to work unchanged. For migration guidance, see [MIGRATION.md](MIGRATION.md).

---

## License

This project is licensed under the Apache 2.0 License. See the [LICENSE](LICENSE) file for the full license text.

Copyright (c) 2025 Giant Swarm GmbH
