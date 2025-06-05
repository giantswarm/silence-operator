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
| **Deprecated Fields** | Includes `TargetTags`, `PostmortemURL` | Cleaned up, only essential fields |
| **Finalizer** | `monitoring.giantswarm.io/silence-protection` | `observability.giantswarm.io/silence-protection` |
| **Controller** | `SilenceReconciler` | `SilenceV2Reconciler` |

## API Changes

### Removed Fields in v1alpha2

```yaml
# v1alpha1 (deprecated fields)
spec:
  targetTags:          # ❌ Removed - was optional
  - name: "example"
    value: "test"
  postmortem_url: "..."  # ❌ Removed - use issue_url instead
```

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
    isRegex: false
    isEqual: true
  owner: "john.doe"
  issue_url: "https://github.com/company/issues/123"
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

For environments with many silences, use this migration script:

```bash
#!/bin/bash
set -euo pipefail

# Configuration
SOURCE_API="monitoring.giantswarm.io/v1alpha1"
TARGET_API="observability.giantswarm.io/v1alpha2"
TARGET_NAMESPACE="${1:-default}"  # Pass namespace as argument

echo "Migrating silences from $SOURCE_API to $TARGET_API in namespace $TARGET_NAMESPACE"

# Get all v1alpha1 silences
kubectl get silences.monitoring.giantswarm.io -o json | jq -r '.items[] | @base64' | while read -r silence; do
    # Decode and extract details
    SILENCE_JSON=$(echo "$silence" | base64 --decode)
    NAME=$(echo "$SILENCE_JSON" | jq -r '.metadata.name')
    MATCHERS=$(echo "$SILENCE_JSON" | jq '.spec.matchers')
    OWNER=$(echo "$SILENCE_JSON" | jq -r '.spec.owner // empty')
    ISSUE_URL=$(echo "$SILENCE_JSON" | jq -r '.spec.issue_url // empty')
    
    echo "Migrating silence: $NAME"
    
    # Create v1alpha2 silence
    kubectl apply -f - <<EOF
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: $NAME
  namespace: $TARGET_NAMESPACE
spec:
  matchers: $MATCHERS
  owner: "$OWNER"
  issue_url: "$ISSUE_URL"
EOF
    
    echo "✅ Created v1alpha2 silence: $NAME in namespace $TARGET_NAMESPACE"
done

echo "🎉 Migration completed!"
echo "⚠️  Remember to test the new silences before removing the old ones."
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
kubectl logs -f deployment/silence-operator -n monitoring | grep -E "(SilenceReconciler|SilenceV2Reconciler)"
```

## Support

For issues during migration:

1. **Check the logs**: `kubectl logs deployment/silence-operator -n monitoring`
2. **Verify CRDs**: `kubectl get crd | grep silences`
3. **Test both APIs**: Create test silences in both v1alpha1 and v1alpha2
4. **Monitor Alertmanager**: Verify silences are actually created in Alertmanager

---

**Note**: Both API versions can coexist safely. The migration can be performed gradually without service interruption.
