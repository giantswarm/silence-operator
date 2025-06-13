#!/bin/bash
# Enhanced migration script with boolean to enum conversion
# Usage: ./migrate-silences.sh [target-namespace] [--dry-run]

set -euo pipefail

# Help function
show_help() {
    cat << EOF
Usage: $0 [target-namespace] [--dry-run|--help]

Migrates v1alpha1 silences to v1alpha2 format with automatic matchers conversion.

Arguments:
  target-namespace    Target namespace for v1alpha2 silences (default: default)
  --dry-run           Show what would be migrated without creating resources
  --help              Show this help message

Features:
  ✅ Automatic boolean-to-enum conversion (isRegex/isEqual → matchType)
  ✅ User annotation/label preservation with system metadata filtering
  ✅ Comprehensive validation and error handling
  ✅ Detailed migration logging

Examples:
  $0 --dry-run                    # Test migration to default namespace
  $0 production --dry-run         # Test migration to production namespace
  $0 monitoring                   # Migrate to monitoring namespace
  $0                             # Migrate to default namespace

For more information, see MIGRATION.md
EOF
}

# Check for help flag
if [[ "${1:-}" == "--help" ]] || [[ "${2:-}" == "--help" ]]; then
    show_help
    exit 0
fi

TARGET_NAMESPACE="${1:-default}"
DRY_RUN=false

# Check for dry-run flag
if [[ "${2:-}" == "--dry-run" ]] || [[ "${1:-}" == "--dry-run" ]]; then
    DRY_RUN=true
    if [[ "${1:-}" == "--dry-run" ]]; then
        TARGET_NAMESPACE="default"
    fi
fi

if [[ "$DRY_RUN" == "true" ]]; then
    echo "🔍 DRY RUN MODE: No resources will be created"
fi

echo "🔄 Migrating v1alpha1 silences to v1alpha2 in namespace: $TARGET_NAMESPACE"
echo "============================================================================"

# Function to convert a single matcher from boolean to enum
convert_matcher_to_enum() {
    local matcher_json="$1"
    
    local name
    local value
    local isRegex
    local isEqual
    
    name=$(echo "$matcher_json" | jq -r '.name')
    value=$(echo "$matcher_json" | jq -r '.value')
    isRegex=$(echo "$matcher_json" | jq -r '.isRegex // false')
    isEqual=$(echo "$matcher_json" | jq -r 'if has("isEqual") then .isEqual else true end')
    
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

# Check if v1alpha2 CRD is installed
echo "🔍 Checking v1alpha2 CRD installation..."
if ! kubectl get crd silences.observability.giantswarm.io &> /dev/null; then
    echo "❌ v1alpha2 CRD (silences.observability.giantswarm.io) is not installed"
    echo "Please install it first:"
    echo "kubectl apply --server-side -f https://raw.githubusercontent.com/giantswarm/silence-operator/main/config/crd/observability.giantswarm.io_silences.yaml"
    exit 1
fi
echo "✅ v1alpha2 CRD is installed"

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
    
    # Extract annotations and labels, excluding Kubernetes and FluxCD system metadata
    # This regex pattern excludes common system annotations/labels
    annotations=$(echo "$silence" | jq '
        .metadata.annotations // {} | 
        with_entries(
            select(
                .key | test("^(kubernetes\\.io|k8s\\.io|config\\.kubernetes\\.io|app\\.kubernetes\\.io|fluxcd\\.io|helm\\.sh|kustomize\\.toolkit\\.fluxcd\\.io|source\\.toolkit\\.fluxcd\\.io|meta\\.helm\\.sh|kubectl\\.kubernetes\\.io|control-plane\\.alpha\\.kubernetes\\.io|node\\.alpha\\.kubernetes\\.io|volume\\.alpha\\.kubernetes\\.io|admission\\.gke\\.io|autopilot\\.gke\\.io|cloud\\.google\\.com|container\\.googleapis\\.com)") | not
            )
        )
    ')
    labels=$(echo "$silence" | jq '
        .metadata.labels // {} | 
        with_entries(
            select(
                .key | test("^(kubernetes\\.io|k8s\\.io|app\\.kubernetes\\.io|pod-template-hash|controller-revision-hash|fluxcd\\.io|helm\\.sh|kustomize\\.toolkit\\.fluxcd\\.io|source\\.toolkit\\.fluxcd\\.io|meta\\.helm\\.sh|kubectl\\.kubernetes\\.io|control-plane\\.alpha\\.kubernetes\\.io|node\\.alpha\\.kubernetes\\.io|volume\\.alpha\\.kubernetes\\.io|admission\\.gke\\.io|autopilot\\.gke\\.io|cloud\\.google\\.com|container\\.googleapis\\.com)") | not
            )
        )
    ')
    
    # Log annotations and labels being copied
    annotation_count=$(echo "$annotations" | jq 'length')
    label_count=$(echo "$labels" | jq 'length')
    
    if [[ "$annotation_count" -gt 0 ]]; then
        echo "   📎 Copying $annotation_count user annotation(s): $(echo "$annotations" | jq -r 'keys | join(", ")')"
    fi
    
    if [[ "$label_count" -gt 0 ]]; then
        echo "   🏷️  Copying $label_count user label(s): $(echo "$labels" | jq -r 'keys | join(", ")')"
    fi
    
    # Create the v1alpha2 silence with preserved user annotations and labels
    # Build metadata object with preserved annotations and labels
    metadata_base=$(jq -n --arg name "$name" --arg namespace "$TARGET_NAMESPACE" \
        '{name: $name, namespace: $namespace}')
    
    # Add annotations if present
    if [[ "$annotation_count" -gt 0 ]]; then
        metadata_base="$(echo "$metadata_base" | jq --argjson annotations "$annotations" \
            '. + {annotations: $annotations}')"
    fi
    
    # Add labels if present
    if [[ "$label_count" -gt 0 ]]; then
        metadata_base="$(echo "$metadata_base" | jq --argjson labels "$labels" \
            '. + {labels: $labels}')"
    fi
    
    # Create the full silence YAML using jq
    silence_yaml="$(jq -n \
        --argjson metadata "$metadata_base" \
        --argjson matchers "$converted_matchers" \
        '{
            apiVersion: "observability.giantswarm.io/v1alpha2",
            kind: "Silence",
            metadata: $metadata,
            spec: {
                matchers: $matchers
            }
        }')"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "   🔍 DRY RUN: Would create v1alpha2 silence in namespace $TARGET_NAMESPACE"
        echo "   📄 Generated YAML:"
        echo "$silence_yaml" | jq -r '.' | sed 's/^/      /'
    else
        echo "   ✨ Creating v1alpha2 silence in namespace $TARGET_NAMESPACE..."
        if echo "$silence_yaml" | kubectl apply -f - 2>/dev/null; then
            echo "   ✅ Successfully created v1alpha2 silence: $name"
        else
            echo "   ❌ Failed to create v1alpha2 silence: $name"
            echo "   📄 Generated content:"
            echo "$silence_yaml" | jq -r '.' | sed 's/^/      /'
            exit 1
        fi
    fi
    echo ""
done

if [[ "$DRY_RUN" == "true" ]]; then
    echo "🔍 DRY RUN completed successfully!"
    echo ""
    echo "To actually perform the migration:"
    echo "1. Run this script without --dry-run: $0 $TARGET_NAMESPACE"
    echo "2. Verify the migrated silences: kubectl get silences.observability.giantswarm.io -n $TARGET_NAMESPACE"
    echo "3. Test that silences work as expected"
    echo "4. Remove old v1alpha1 silences when confident: kubectl delete silences.monitoring.giantswarm.io --all"
else
    echo "🎉 Migration completed successfully!"
    echo ""
    echo "Next steps:"
    echo "1. Verify the migrated silences: kubectl get silences.observability.giantswarm.io -n $TARGET_NAMESPACE"
    echo "2. Test that silences work as expected"
    echo "3. Remove old v1alpha1 silences when confident: kubectl delete silences.monitoring.giantswarm.io --all"
fi
