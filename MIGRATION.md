# Migration Guide: v1alpha1 to v1alpha2

This guide helps you migrate from the legacy `monitoring.giantswarm.io/v1alpha1` cluster-scoped Silence API to the new `observability.giantswarm.io/v1alpha2` namespace-scoped API.

## Overview

The silence-operator now supports two API versions:

- **v1alpha1** (`monitoring.giantswarm.io/v1alpha1`) - **Cluster-scoped** (legacy, deprecated)
- **v1alpha2** (`observability.giantswarm.io/v1alpha2`) - **Namespace-scoped** (recommended)

## Key Differences

| Aspect | v1alpha1 | v1alpha2 |
|--------|----------|----------|
| **API Group** | `monitoring.giantswarm.io` | `observability.giantswarm.io` |
| **Scope** | Cluster-scoped | Namespace-scoped |
| **Matcher Fields** | `isRegex: bool`, `isEqual: bool` | `matchType: string` (enum) |
| **Validation** | Basic validation | Enhanced validation with field size limits |
| **Deprecated Fields** | Includes `targetTags`, `owner`, `issue_url`, `postmortem_url` | Cleaned up, only essential fields |
| **Finalizer** | `monitoring.giantswarm.io/silence-protection` | `observability.giantswarm.io/silence-protection` |
| **Controller** | `SilenceReconciler` | `SilenceV2Reconciler` |

## API Changes

### Matcher Field Changes in v1alpha2

The most significant change in v1alpha2 is the replacement of boolean matcher fields with an enum:

```yaml
# v1alpha1 (old boolean approach)
spec:
  matchers:
  - name: "alertname"
    value: "HighCPU"
    isRegex: false    # ❌ Removed in v1alpha2
    isEqual: true     # ❌ Removed in v1alpha2

# v1alpha2 (new enum approach)  
spec:
  matchers:
  - name: "alertname"
    value: "HighCPU"
    matchType: "="    # ✅ New enum field using Alertmanager symbols
```

#### MatchType Values

| v1alpha1 Boolean Combination | v1alpha2 MatchType | Description |
|------------------------------|-------------------|-------------|
| `isRegex: false, isEqual: true` | `"="` | Exact string match |
| `isRegex: false, isEqual: false` | `"!="` | Exact string non-match |
| `isRegex: true, isEqual: true` | `"=~"` | Regex match |
| `isRegex: true, isEqual: false` | `"!~"` | Regex non-match |

### Removed Fields in v1alpha2

The v1alpha2 API removes several fields that were present in v1alpha1 for a cleaner, more focused API surface:

```yaml
# v1alpha1 (deprecated fields removed in v1alpha2)
spec:
  targetTags:          # ❌ Removed - legacy field, not commonly used
  - name: "example"
    value: "test"
  owner: "username"     # ❌ Removed
  postmortem_url: "..." # ❌ Removed
  issue_url: "..."      # ❌ Removed
```

### Enhanced Validation in v1alpha2

v1alpha2 includes comprehensive validation:

- **Matcher name**: Required, 1-256 characters
- **Matcher value**: Required, max 1024 characters  
- **Matchers array**: At least 1 matcher required
- **MatchType**: Must be one of `=`, `!=`, `=~`, `!~` (defaults to `=`)

## Migration Strategies

### Strategy 1: Gradual Migration (Recommended)

This approach allows you to migrate gradually while maintaining existing silences.

#### Step 1: Deploy Updated Operator

Ensure you're running silence-operator version that supports both APIs (v0.17.0+).

```bash
# Check current version
kubectl get deployment silence-operator -n monitoring -o jsonpath='{.spec.template.spec.containers[0].image}'

# Update via Helm (example)
helm upgrade silence-operator giantswarm/silence-operator --version 0.17.0 -n monitoring
```

#### Step 2: Create v1alpha2 Silences in Target Namespaces

For each existing v1alpha1 silence, create a corresponding v1alpha2 silence:

```bash
# List existing v1alpha1 silences
kubectl get silences.monitoring.giantswarm.io

# Create v1alpha2 equivalent in target namespace
kubectl apply -f - <<EOF
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: example-silence
  namespace: production  # 🔥 Now namespace-scoped!
spec:
  matchers:
  - name: "alertname"
    value: "HighCPUUsage"
    matchType: "="       # ✅ New enum field instead of isRegex/isEqual
EOF
```

#### Step 3: Verify v1alpha2 Silences Work

```bash
kubectl get silences.observability.giantswarm.io -n production
kubectl describe silence example-silence -n production
```

#### Step 4: Remove v1alpha1 Silences

Once v1alpha2 silences are working correctly:

```bash
# Remove old v1alpha1 silences
kubectl delete silences.monitoring.giantswarm.io example-silence
```

### Strategy 2: Bulk Migration Script

For environments with many silences, use the provided migration script:

✅ **Automated Conversion**: This script automatically converts boolean fields (`isRegex`/`isEqual`) to the new `matchType` enum format. No manual intervention required!

```bash
# Run the migration script (located in hack/migrate-silences.sh)
./hack/migrate-silences.sh [target-namespace] [--dry-run]

# Example: Test migration to production namespace (dry-run)
./hack/migrate-silences.sh production --dry-run

# Example: Migrate all v1alpha1 silences to the production namespace
./hack/migrate-silences.sh production

# Example: Migrate to default namespace
./hack/migrate-silences.sh
```

The script will:
1. Fetch all existing v1alpha1 silences
2. Convert boolean matcher fields to enum format automatically
3. Create equivalent v1alpha2 silences in the target namespace
4. **Intelligently preserve user metadata** while filtering out system annotations/labels
5. Provide detailed output of the conversion process

### Metadata Filtering During Migration

The migration script **automatically preserves user-defined annotations and labels** while filtering out Kubernetes and FluxCD system metadata:

#### ✅ **Preserved** (User Metadata):
- `motivation` - User-defined reasoning for the silence
- `valid-until` - User-defined expiry date
- `issue` - User-defined issue tracker links  
- `app.example.com/*` - Custom application labels
- `team.company.com/*` - Custom team labels
- Any other user-defined annotations/labels

#### ❌ **Filtered Out** (System Metadata):
- `kubernetes.io/*` - Core Kubernetes metadata
- `k8s.io/*` - Kubernetes ecosystem 
- `config.kubernetes.io/*` - Kubernetes configuration origin
- `app.kubernetes.io/*` - Kubernetes app labeling
- `fluxcd.io/*` - FluxCD system metadata
- `helm.sh/*` - Helm metadata
- `kustomize.toolkit.fluxcd.io/*` - Kustomize FluxCD labels
- `source.toolkit.fluxcd.io/*` - Source FluxCD
- `meta.helm.sh/*` - Helm meta
- `kubectl.kubernetes.io/*` - kubectl metadata
- `control-plane.alpha.kubernetes.io/*` - Control plane
- `node.alpha.kubernetes.io/*` - Node metadata
- `volume.alpha.kubernetes.io/*` - Volume metadata
- `pod-template-hash`, `controller-revision-hash` - Kubernetes controller metadata
- Cloud provider specific annotations/labels (GKE, etc.)

#### Example Filtering:

**Real-world example from `common-jobscrapingfailure` silence:**

```yaml
# Original v1alpha1 metadata
metadata:
  annotations:
    config.kubernetes.io/origin: |        # ❌ FILTERED OUT (system annotation)
      path: bases/silences/jobscrapingfailure.yaml
      repo: https://github.com/giantswarm/management-cluster-bases
      ref: main
    motivation: "We did a review of jobs failing everywhere, let's give teams time to manage them."  # ✅ PRESERVED
    valid-until: "2025-07-29"              # ✅ PRESERVED
  labels:
    kustomize.toolkit.fluxcd.io/name: silences          # ❌ FILTERED OUT (FluxCD system label)
    kustomize.toolkit.fluxcd.io/namespace: flux-giantswarm  # ❌ FILTERED OUT (FluxCD system label)
    app.example.com/component: monitoring               # ✅ WOULD BE PRESERVED (user label)

# Migrated v1alpha2 metadata  
metadata:
  name: common-jobscrapingfailure
  namespace: production
  annotations:
    motivation: "We did a review of jobs failing everywhere, let's give teams time to manage them."  # ✅ PRESERVED
    valid-until: "2025-07-29"              # ✅ PRESERVED
  labels:
    app.example.com/component: monitoring   # ✅ WOULD BE PRESERVED (user label)
```

#### Testing the Filtering

Run the script in dry-run mode to see what gets filtered:

```bash
./hack/migrate-silences.sh --dry-run
```

Look for lines like:
```
📎 Copying 2 user annotation(s): motivation, valid-until
🏷️  Copying 1 user label(s): app.example.com/component
```

## Practical Conversion Examples

Here are real-world examples of converting from v1alpha1 to v1alpha2:

### Example 1: Exact String Match

```yaml
# v1alpha1
apiVersion: monitoring.giantswarm.io/v1alpha1
kind: Silence
metadata:
  name: silence-deployment-alerts
spec:
  matchers:
  - name: "alertname"
    value: "DeploymentReplicasMismatch"
    isRegex: false
    isEqual: true

# v1alpha2 equivalent
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: silence-deployment-alerts
  namespace: production
spec:
  matchers:
  - name: "alertname"
    value: "DeploymentReplicasMismatch"
    matchType: "="
```

### Example 2: Regex Pattern Match

```yaml
# v1alpha1
apiVersion: monitoring.giantswarm.io/v1alpha1
kind: Silence
metadata:
  name: silence-cpu-alerts
spec:
  matchers:
  - name: "alertname"
    value: "High.*CPU.*"
    isRegex: true
    isEqual: true

# v1alpha2 equivalent
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: silence-cpu-alerts
  namespace: production
spec:
  matchers:
  - name: "alertname"
    value: "High.*CPU.*"
    matchType: "=~"
```

### Example 3: Exclude Specific Values

```yaml
# v1alpha1
apiVersion: monitoring.giantswarm.io/v1alpha1
kind: Silence
metadata:
  name: silence-non-critical-alerts
spec:
  matchers:
  - name: "severity"
    value: "critical"
    isRegex: false
    isEqual: false

# v1alpha2 equivalent
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: silence-non-critical-alerts
  namespace: production
spec:
  matchers:
  - name: "severity"
    value: "critical"
    matchType: "!="
```

## RBAC Considerations

With namespace-scoped resources, you can implement more granular RBAC:

```yaml
# Example: Team-specific RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: platform-team
  name: silence-manager
rules:
- apiGroups: ["observability.giantswarm.io"]
  resources: ["silences"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: platform-team-silence-managers
  namespace: platform-team
subjects:
- kind: User
  name: alice@company.com
  apiGroup: rbac.authorization.k8s.io
- kind: User  
  name: bob@company.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: silence-manager
  apiGroup: rbac.authorization.k8s.io
```

## Monitoring the Migration

### Check Both API Versions

```bash
# List all v1alpha1 silences (cluster-scoped)
kubectl get silences.monitoring.giantswarm.io

# List all v1alpha2 silences (all namespaces)
kubectl get silences.observability.giantswarm.io --all-namespaces

# Compare counts
echo "v1alpha1 count: $(kubectl get silences.monitoring.giantswarm.io --no-headers | wc -l)"
echo "v1alpha2 count: $(kubectl get silences.observability.giantswarm.io --all-namespaces --no-headers | wc -l)"
```

### Monitor Controller Logs

```bash
# Watch both controllers
kubectl logs -f deployment/silence-operator -n monitoring
```

## Testing During Migration

### Dual Controller Testing

During migration, both controllers run simultaneously. You can verify both are working:

```bash
# Test v1alpha1 controller
kubectl apply -f - <<EOF
apiVersion: monitoring.giantswarm.io/v1alpha1
kind: Silence
metadata:
  name: test-v1alpha1
spec:
  matchers:
  - name: "alertname"
    value: "TestAlert"
    isRegex: false
    isEqual: true
EOF

# Test v1alpha2 controller  
kubectl apply -f - <<EOF
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: test-v1alpha2
  namespace: default
spec:
  matchers:
  - name: "alertname"
    value: "TestAlert"
    matchType: "="
EOF
```

### Verification Commands

```bash
# Check both resources exist
kubectl get silences.monitoring.giantswarm.io
kubectl get silences.observability.giantswarm.io --all-namespaces

# Verify finalizers are properly set
kubectl get silence test-v1alpha1 -o jsonpath='{.metadata.finalizers}'
kubectl get silence test-v1alpha2 -n default -o jsonpath='{.metadata.finalizers}'

# Check controller logs for both APIs
kubectl logs -f deployment/silence-operator -n monitoring
```


## Support

For issues during migration:

1. **Check the logs**: `kubectl logs deployment/silence-operator -n monitoring`
2. **Verify CRDs**: `kubectl get crd | grep silences`
3. **Test both APIs**: Create test silences in both v1alpha1 and v1alpha2
4. **Monitor Alertmanager**: Verify silences are actually created in Alertmanager

---

**Note**: Both API versions can coexist safely. The migration can be performed gradually without service interruption.
