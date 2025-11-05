[![CircleCI](https://circleci.com/gh/giantswarm/silence-operator.svg?&style=shield)](https://circleci.com/gh/giantswarm/silence-operator)
[![Docker Repository on Quay](https://quay.io/repository/giantswarm/silence-operator/status "Docker Repository on Quay")](https://quay.io/repository/giantswarm/silence-operator)

# silence-operator

The Silence Operator automates the management of [Alertmanager](https://github.com/prometheus/alertmanager) [silences](https://prometheus.io/docs/alerting/latest/alertmanager/#silences) using Kubernetes Custom Resources. This allows you to define and manage silences declaratively, just like other Kubernetes objects, integrating them into your GitOps workflows.

## Prerequisites

- Kubernetes 1.25+
- Helm 3+

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

_See [`helm repo`](https://helm.sh/docs/helm/helm_repo/) for command documentation._

```console
helm install [RELEASE_NAME] giantswarm/silence-operator
```

**Note**: The operator supports both API versions for backward compatibility. New deployments should use the namespace-scoped `observability.giantswarm.io/v1alpha2` API. See [MIGRATION.md](MIGRATION.md) for migration guidance.

```console
helm install [RELEASE_NAME] giantswarm/silence-operator \
  --set alertmanagerAddress="http://my-alertmanager:9093" \
  --set silenceSelector="environment=production" \
  --set namespaceSelector="team=platform"
```

_See [helm install](https://helm.sh/docs/helm/helm_install/) for command documentation._

## CRDs

CRDs are **not created automatically** by this chart and should be manually deployed:

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

_See [helm upgrade](https://helm.sh/docs/helm/helm_upgrade/) for command documentation._

CRDs are not removed by default and should be manually cleaned up:

```console
# Remove legacy cluster-scoped CRD
kubectl delete crd silences.monitoring.giantswarm.io

# Remove namespace-scoped CRD
kubectl delete crd silences.observability.giantswarm.io
```

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

## Configuration

The silence-operator can be configured through Helm values to control which resources it processes:

### Silence Selector

Filter which `Silence` custom resources the operator processes based on their labels. This applies to both v1alpha1 and v1alpha2 APIs.

```yaml
# values.yaml
silenceSelector: "environment=production,tier=frontend"
```

**Examples:**
- `"environment=production"` - Only process silences with `environment=production` label
- `"team=platform,tier=monitoring"` - Only process silences with both labels
- `"environment in (production,staging)"` - Process silences with environment in the specified set
- `""` - Process all silence resources (default)

### Namespace Selector

Restrict which namespaces the v2 controller watches for `Silence` CRs. This only applies to the namespace-scoped `observability.giantswarm.io/v1alpha2` API.

```yaml
# values.yaml
namespaceSelector: "environment=production"
```

**Examples:**
- `"environment=production"` - Only watch namespaces labeled with `environment=production`
- `"team=platform,tier=monitoring"` - Only watch namespaces with both labels  
- `"team notin (test,staging)"` - Watch namespaces except those with specified team labels
- `""` - Watch all namespaces (default)

**Note:** The namespace selector provides an additional layer of filtering for the v2 controller, allowing you to restrict monitoring to specific namespace subsets. The v1 controller continues to process all cluster-scoped v1alpha1 resources regardless of this setting.

### Complete Configuration Example

```yaml
# values.yaml
alertmanagerAddress: "http://alertmanager.monitoring.svc.cluster.local:9093"
alertmanagerAuthentication: true
alertmanagerDefaultTenant: "my-tenant"

# Only process silences for production workloads
silenceSelector: "environment=production"

# Only watch namespaces managed by the platform team
namespaceSelector: "team=platform,environment in (production,staging)"
```

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
- **New API group**: Uses `observability.giantswarm.io` instead of `monitoring.giantswarm.io`
- **Simplified matcher syntax**: Uses enum-based `matchType` field (`=`, `!=`, `=~`, `!~`) instead of boolean `isRegex`/`isEqual` fields
- **Streamlined spec**: Removes legacy fields (`targetTags`, `owner`, `issue_url`, `postmortem_url`) for a cleaner API surface
- **Enhanced validation**: Includes stricter field validation and length limits for better error handling

**Migration steps:**
1. Deploy the new v1alpha2 CRD (both CRDs can coexist)
2. Create equivalent v1alpha2 Silence resources in appropriate namespaces
3. Verify the new silences are working correctly
4. Remove old v1alpha1 resources

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
The operator can be configured to only process `Silence` CRs that match a specific label selector. This is done by setting the `silenceSelector` value in the Helm chart (e.g., `silenceSelector: "team=alpha"`). If left empty or not provided, the operator will process all `Silence` CRs in the cluster.

**Filtering Options:**

The operator provides two filtering mechanisms:

1. **Silence Filtering (`silenceSelector`):** Filters which `Silence` CRs the operator processes based on their labels. This applies to both v1alpha1 and v1alpha2 APIs. Set via `silenceSelector` in the Helm chart (e.g., `silenceSelector: "team=alpha"`). If empty, all `Silence` CRs are processed.

2. **Namespace Filtering (`namespaceSelector`):** Restricts which namespaces the v2 controller watches for `Silence` CRs. Only applies to the namespace-scoped v1alpha2 API. Set via `namespaceSelector` in the Helm chart (e.g., `namespaceSelector: "environment=production"`). If empty, the v2 controller watches all namespaces.

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
    isEqual: true
  - name: severity
    value: critical
    isRegex: false
    isEqual: true
  owner: example-user
  issue_url: https://github.com/example/issue/123
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
  - name: severity
    value: critical
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

## Mimir Multi-Tenancy Configuration

The silence-operator supports multi-tenant configurations for Mimir Alermanager, allowing different teams or environments to manage their own silences independently.

### Tenancy Configuration

Multi-tenancy can be enabled via Helm values or command-line flags:

**Helm Configuration:**
```yaml
# Enable tenancy support
tenancy:
  enabled: true
  labelKey: "observability.giantswarm.io/tenant"
  defaultTenant: "default"

# Legacy configuration (deprecated but supported)
alertmanagerDefaultTenant: "legacy-tenant"
```

**Command-line Flags:**
```bash
--tenancy-enabled=true
--tenancy-label-key="observability.giantswarm.io/tenant"
--tenancy-default-tenant="default"

# Legacy flag (deprecated but supported)
--alertmanager-default-tenant-id="legacy-tenant"
```

### How Tenancy Works

When tenancy is enabled, the operator:

1. **Extracts tenant information** from the Silence resource using the configured label key
2. **Uses tenant-specific Alertmanager clients** that include the `X-Scope-OrgID` header
3. **Falls back to the default tenant** when no tenant label is found on the resource

**Example with tenant label:**
```yaml
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: team-alpha-silence
  namespace: team-alpha
  labels:
    observability.giantswarm.io/tenant: "team-alpha"
spec:
  matchers:
  - name: team
    value: alpha
    matchType: "="
```

**Example without tenant label (uses default tenant):**
```yaml
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: shared-silence
  namespace: monitoring
spec:
  matchers:
  - name: severity
    value: warning
    matchType: "="
```

### Backward Compatibility

The operator maintains full backward compatibility with existing configurations:

- Setting `alertmanagerDefaultTenant` automatically enables tenancy and uses that value as the default tenant
- Existing silences without tenant labels continue to work unchanged
- The legacy `alertmanagerDefaultTenant` setting is preserved and mapped to the new tenancy configuration

### Multi-Tenant Alertmanager Setup

For proper multi-tenancy, your Mimir Alertmanager should be configured to support tenant-specific configurations. This typically involves:

1. **Tenant-aware routing** based on the `X-Scope-OrgID` header
2. **Separate silence storage** per tenant
3. **Isolated notification configurations** per tenant

Consult your Mimir Alertmanager documentation for specific multi-tenancy setup instructions.

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
  - `Alertmanager`: Concrete implementation

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

## Architecture Documentation

### Project Structure

```
silence-operator/
├── api/                             # API definitions and schemas
│   ├── v1alpha1/                   # Legacy cluster-scoped API
│   └── v1alpha2/                   # New namespace-scoped API
├── internal/controller/            # Kubernetes controllers
│   ├── silence_controller.go       # v1alpha1 controller (legacy)
│   ├── silence_v2_controller.go    # v1alpha2 controller (recommended)
│   └── testutils/                  # Test utilities and mocks
├── pkg/                            # Reusable packages
│   ├── alertmanager/              # Alertmanager client implementation
│   └── service/                   # Business logic layer
├── config/                        # Kubernetes manifests and CRDs
├── helm/                          # Helm chart for deployment
└── docs/                          # Documentation
```

### Design Principles

1. **Clean Architecture**: Clear separation between controllers, services, and external clients
2. **Dual API Support**: Maintains backward compatibility while providing improved v2 API
3. **Dependency Injection**: Services are injected into controllers for better testability
4. **Interface-Based Design**: Uses interfaces for external dependencies (Alertmanager client)
5. **Shared Business Logic**: Common operations are handled by the service layer

### Controller Responsibilities

**SilenceReconciler (v1alpha1)**:
- Manages cluster-scoped silences
- Handles legacy boolean matcher fields (`isRegex`, `isEqual`)
- Maintains backward compatibility

**SilenceV2Reconciler (v1alpha2)**:
- Manages namespace-scoped silences
- Uses enum-based matcher types (`matchType: =, !=, =~, !~`)
- Provides enhanced validation and user experience

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
