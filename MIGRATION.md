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
| **Deprecated Fields** | Includes `TargetTags`, `PostmortemURL` | Cleaned up, only essential fields |
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

```yaml
# v1alpha1 (deprecated fields)
spec:
  targetTags:          # ❌ Removed - was optional
  - name: "example"
    value: "test"
  postmortem_url: "..."  # ❌ Removed - use issue_url instead
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

For environments with many silences, use this migration script:

✅ **Automated Conversion**: This script automatically converts boolean fields (`isRegex`/`isEqual`) to the new `matchType` enum format. No manual intervention required!

```bash
#!/bin/bash
# Enhanced migration script with boolean to enum conversion
# Usage: ./migrate_silences.sh [target-namespace]

set -euo pipefail

TARGET_NAMESPACE="${1:-default}"

echo "🔄 Migrating v1alpha1 silences to v1alpha2 in namespace: $TARGET_NAMESPACE"
echo "============================================================================"

# Function to convert a single matcher from boolean to enum
convert_matcher_to_enum() {
    local matcher_json="$1"
    
    local name=$(echo "$matcher_json" | jq -r '.name')
    local value=$(echo "$matcher_json" | jq -r '.value')
    local isRegex=$(echo "$matcher_json" | jq -r '.isRegex // false')
    local isEqual=$(echo "$matcher_json" | jq -r 'if has("isEqual") then .isEqual else true end')
    
    # Convert boolean combination to enum
    local matchType
    case "${isRegex}-${isEqual}" in
        "false-true")  matchType="=" ;;
        "false-false") matchType="!=" ;;
        "true-true")   matchType="=~" ;;
        "true-false")  matchType="!~" ;;
        *)             matchType="=" ;;  # Default fallback
    esac
    
    # Return new matcher format
    jq -n --arg name "$name" --arg value "$value" --arg matchType "$matchType" \
        '{name: $name, value: $value, matchType: $matchType}'
}

# Check prerequisites
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is required but not found"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "❌ jq is required but not found"
    exit 1
fi

# Get v1alpha1 silences
echo "📋 Fetching v1alpha1 silences..."
if ! kubectl get silences.monitoring.giantswarm.io &> /dev/null; then
    echo "❌ No v1alpha1 silences found or CRD not installed"
    exit 1
fi

silences_json=$(kubectl get silences.monitoring.giantswarm.io -o json)
silence_count=$(echo "$silences_json" | jq '.items | length')

if [[ "$silence_count" == "0" ]]; then
    echo "ℹ️  No v1alpha1 silences found to migrate"
    exit 0
fi

echo "📝 Found $silence_count v1alpha1 silence(s) to migrate"
echo ""

# Process each silence
echo "$silences_json" | jq -r '.items[] | @base64' | while read -r encoded_silence; do
    silence=$(echo "$encoded_silence" | base64 --decode)
    
    name=$(echo "$silence" | jq -r '.metadata.name')
    echo "🔄 Processing silence: $name"
    
    # Convert all matchers
    converted_matchers="[]"
    matchers=$(echo "$silence" | jq '.spec.matchers')
    matcher_count=$(echo "$matchers" | jq 'length')
    
    for i in $(seq 0 $((matcher_count - 1))); do
        original_matcher=$(echo "$matchers" | jq ".[$i]")
        converted_matcher=$(convert_matcher_to_enum "$original_matcher")
        converted_matchers=$(echo "$converted_matchers" | jq ". += [$converted_matcher]")
        
        # Log conversion
        matcher_name=$(echo "$original_matcher" | jq -r '.name')
        old_isRegex=$(echo "$original_matcher" | jq -r '.isRegex // false')
        old_isEqual=$(echo "$original_matcher" | jq -r 'if has("isEqual") then .isEqual else true end')
        new_matchType=$(echo "$converted_matcher" | jq -r '.matchType')
        
        echo "   📝 $matcher_name: isRegex=$old_isRegex, isEqual=$old_isEqual → matchType='$new_matchType'"
    done
    
    # Create the v1alpha2 silence
    echo "   ✨ Creating v1alpha2 silence in namespace $TARGET_NAMESPACE..."
    
    kubectl apply -f - <<EOF
apiVersion: observability.giantswarm.io/v1alpha2
kind: Silence
metadata:
  name: $name
  namespace: $TARGET_NAMESPACE
spec:
  matchers: $converted_matchers
EOF
    
    echo "   ✅ Successfully created v1alpha2 silence: $name"
    echo ""
done

echo "🎉 Migration completed successfully!"
echo ""
echo "Next steps:"
echo "1. Verify the migrated silences: kubectl get silences.observability.giantswarm.io -n $TARGET_NAMESPACE"
echo "2. Test that silences work as expected"
echo "3. Remove old v1alpha1 silences when confident: kubectl delete silences.monitoring.giantswarm.io <name>"
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
