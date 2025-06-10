[![CircleCI](https://circleci.com/gh/giantswarm/silence-operator.svg?&style=shield)](https://circleci.com/gh/giantswarm/silence-operator)
[![Docker Repository on Quay](https://quay.io/repository/giantswarm/silence-operator/status "Docker Repository on Quay")](https://quay.io/repository/giantswarm/silence-operator)

# silence-operator

The Silence Operator automates the management of [Alertmanager](https://github.com/prometheus/alertmanager) [silences](https://prometheus.io/docs/alerting/latest/alertmanager/#silences) using Kubernetes Custom Resources. This allows you to define and manage silences declaratively, just like other Kubernetes objects, integrating them into your GitOps workflows.

## Prerequisites

- Kubernetes 1.25+
- Helm 3+

## Get Helm Repository Info

```console
helm repo add giantswarm https://giantswarm.github.io/control-plane-catalog/
helm repo update
```

_See [`helm repo`](https://helm.sh/docs/helm/helm_repo/) for command documentation._

## Install Helm Chart

```console
helm install [RELEASE_NAME] giantswarm/silence-operator
```

_See [helm install](https://helm.sh/docs/helm/helm_install/) for command documentation._


CRDs are not created by this chart and should be manually deployed:

```console
# For existing cluster-scoped silences (legacy)
kubectl apply --server-side -f https://raw.githubusercontent.com/giantswarm/silence-operator/main/config/crd/monitoring.giantswarm.io_silences.yaml

# For new namespace-scoped silences (recommended)
kubectl apply --server-side -f https://raw.githubusercontent.com/giantswarm/silence-operator/main/config/crd/observability.giantswarm.io_silences.yaml
```

**Note**: The operator supports both API versions for backward compatibility. New deployments should use the namespace-scoped `observability.giantswarm.io/v1alpha2` API. See [MIGRATION.md](MIGRATION.md) for migration guidance.

## Uninstall Helm Chart

```console
helm uninstall [RELEASE_NAME]
```

This removes all the Kubernetes components associated with the chart and deletes the release.

_See [helm uninstall](https://helm.sh/docs/helm/helm_uninstall/) for command documentation._

CRDs are not removed by default and should be manually cleaned up:

```console
# Remove legacy cluster-scoped CRD
kubectl delete crd silences.monitoring.giantswarm.io

# Remove namespace-scoped CRD
kubectl delete crd silences.observability.giantswarm.io
```

## Upgrading Chart

```console
helm upgrade [RELEASE_NAME] giantswarm/silence-operator
```

CRDs should be manually updated:

```
# Update legacy cluster-scoped CRD (if using v1alpha1)
kubectl apply --server-side -f https://raw.githubusercontent.com/giantswarm/silence-operator/main/config/crd/monitoring.giantswarm.io_silences.yaml

# Update namespace-scoped CRD (if using v1alpha2)
kubectl apply --server-side -f https://raw.githubusercontent.com/giantswarm/silence-operator/main/config/crd/observability.giantswarm.io_silences.yaml
```

_See [helm upgrade](https://helm.sh/docs/helm/helm_upgrade/) for command documentation._

## Overview

### API Versions and Migration

The silence-operator supports two API versions:

- **`monitoring.giantswarm.io/v1alpha1`** - Legacy cluster-scoped API (deprecated)
- **`observability.giantswarm.io/v1alpha2`** - New namespace-scoped API (recommended)

**Migration Notice**: The v1alpha1 API is deprecated but remains fully supported for backward compatibility. New deployments should use the v1alpha2 API for better multi-tenancy and namespace isolation. Existing v1alpha1 silences continue to work unchanged. For migration guidance, see [MIGRATION.md](MIGRATION.md).

### Migration from v1alpha1 to v1alpha2

If you're currently using the cluster-scoped `monitoring.giantswarm.io/v1alpha1` API, we recommend migrating to the namespace-scoped `observability.giantswarm.io/v1alpha2` API for improved multi-tenancy and security isolation.

**Key differences in v1alpha2:**
- **Namespace-scoped**: Silences are scoped to specific namespaces instead of cluster-wide
- **Enhanced status**: Comprehensive status tracking with conditions and phase information
- **Additional fields**: Support for `owner` and `issue_url` fields for better traceability
- **New API group**: Uses `observability.giantswarm.io` instead of `monitoring.giantswarm.io`

**Migration steps:**
1. Deploy the new v1alpha2 CRD (both CRDs can coexist)
2. Create equivalent v1alpha2 Silence resources in appropriate namespaces
3. Verify the new silences are working correctly
4. Remove old v1alpha1 resources
5. Eventually remove the v1alpha1 CRD when no longer needed

For detailed migration instructions and examples, see [MIGRATION.md](MIGRATION.md).

### CustomResourceDefinition

The silence-operator monitors the Kubernetes API server for changes
to `Silence` objects and ensures that the current Alertmanager silences match these objects.
The Operator reconciles the `Silence` [Custom Resource Definition (CRD)][crd] which
supports two API versions:

- **v1alpha1** (legacy): [monitoring.giantswarm.io_silences.yaml](config/crd/bases/monitoring.giantswarm.io_silences.yaml)
- **v1alpha2** (recommended): [observability.giantswarm.io_silences.yaml](config/crd/bases/observability.giantswarm.io_silences.yaml)

The v1alpha1 CRD is deployed via [management-cluster-bases](https://github.com/giantswarm/management-cluster-bases/blob/9e17d416dd324e07d7784054237302707ba42dc3/bases/crds/giantswarm/kustomization.yaml#L6C1-L7C1) repository.

[crd]: https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions/
[silence-crd]: api/v1alpha1/silence_types.go

### How does it work

Deployment runs the Kubernetes controller, which reconciles `Silence` CRs.

The operator follows a layered architecture:

- **Controller Layer** (`internal/controller/`): Handles Kubernetes-specific concerns such as CR reconciliation, finalizers, and status updates
- **Service Layer** (`pkg/service/`): Contains business logic for silence synchronization, including creation, updates, and deletion
- **Alertmanager Client** (`pkg/alertmanager/`): Provides interface and implementation for Alertmanager API interactions

This separation ensures clean code organization, improved testability, and easier maintenance.

Sample CR:

**v1alpha1 (legacy, cluster-scoped):**
```yaml
apiVersion: monitoring.giantswarm.io/v1alpha1
kind: Silence
metadata:
  name: test-silence1
spec:
  matchers:
  - name: cluster
    value: test
    isRegex: false
```

**v1alpha2 (recommended, namespace-scoped):**
```yaml
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: test-silence1
  namespace: my-namespace
spec:
  matchers:
  - name: cluster
    value: test
    matchType: "="
```

- `matchers` field corresponds to the Alertmanager silence `matchers` each of which consists of:
  - `name` - name of tag on an alert to match
  - `value` - fixed string or expression to match against the value of the tag named by `name` above on an alert
  - `matchType` - the type of matching to perform using Alertmanager operator symbols:
    - `"="` - exact string match
    - `"!="` - exact string non-match
    - `"=~"` - regex match
    - `"!~"` - regex non-match

## Architecture

The silence-operator follows a clean architecture pattern with clear separation of concerns:

### Components

- **Controllers**: Handle Kubernetes-specific reconciliation logic and lifecycle management
  - `SilenceReconciler`: Manages v1alpha1 cluster-scoped silences (legacy)
  - `SilenceV2Reconciler`: Manages v1alpha2 namespace-scoped silences (recommended)
- **Service Layer**: Contains business logic agnostic to Kubernetes concepts
  - `SilenceService`: Core business logic for creating, updating, and deleting silences
- **Alertmanager Client**: Handles communication with Alertmanager API
  - `alertmanager.Client`: Interface for Alertmanager operations
  - `AlertManager`: Concrete implementation

### Data Flow

1. **Conversion**: Controllers convert Kubernetes CRs to `alertmanager.Silence` objects using `getSilenceFromCR()` methods
2. **Business Logic**: Controllers call `SilenceService` methods (`CreateOrUpdateSilence`, `DeleteSilence`)
3. **Alertmanager Operations**: Service layer uses the `alertmanager.Client` interface to interact with Alertmanager
4. **Error Handling**: Simple error returns propagate back through the layers for Kubernetes to handle retries

### Dependency Injection

The service is instantiated in `main.go` and injected into controllers via constructor dependency injection, enabling:
- Shared business logic between v1alpha1 and v1alpha2 controllers
- Easier testing through interface mocking
- Clear separation between Kubernetes concerns and business logic

## Getting the Project

Download the latest release:
https://github.com/giantswarm/silence-operator/releases/latest

Clone the git repository: https://github.com/giantswarm/silence-operator.git

Download the latest docker image from here:
https://quay.io/repository/giantswarm/silence-operator

### How to build

Build the standard way.

```
go build github.com/giantswarm/silence-operator
```

## Contributing & Reporting Bugs

See [CONTRIBUTING](CONTRIBUTING.md) for details on submitting patches, the
contribution workflow as well as reporting bugs.

For security issues, please see [the security policy](SECURITY.md).

## License

This project is licensed under the Apache 2.0 License. See the [LICENSE](LICENSE) file for the full license text.

Copyright (c) 2025 Giant Swarm GmbH
