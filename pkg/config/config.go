package config

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// WebhookConfig holds configuration for the mutating admission webhook.
// The webhook is active only when at least one CELRule is defined.
type WebhookConfig struct {
	// CELRules are conditional mutation rules evaluated using CEL expressions.
	CELRules []CELRule `json:"celRules"`
}

// IsEnabled returns true when the webhook has at least one rule to enforce.
func (w WebhookConfig) IsEnabled() bool {
	return len(w.CELRules) > 0
}

// MatcherSpec describes a single Alertmanager matcher to inject into a Silence.
type MatcherSpec struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	MatchType string `json:"matchType"` // =, !=, =~, !~
}

// CELRule defines a conditional mutation applied when the CEL Condition evaluates to true.
// The Condition expression receives an "object" variable containing the Silence as a JSON map.
// An empty Condition always matches, making it equivalent to an unconditional forced rule.
//
// To replicate the kyverno-policies-observability Silence policy, use:
//
//	celRules:
//	- name: exclude-heartbeat
//	  condition: ""
//	  matchers:
//	  - name: alertname
//	    value: Heartbeat
//	    matchType: "!="
//	- name: exclude-all-pipelines
//	  condition: ""
//	  matchers:
//	  - name: all_pipelines
//	    value: "true"
//	    matchType: "!="
type CELRule struct {
	// Name is a human-readable identifier for the rule (used in log and error messages).
	Name string `json:"name"`
	// Condition is a CEL expression returning bool. Empty string means always apply.
	// The "object" variable exposes the full Silence resource as a map, e.g.:
	//   object.metadata.namespace == "production"
	//   "team" in object.metadata.labels
	Condition string `json:"condition"`
	// Matchers to inject when Condition is true (idempotent — no duplicates).
	Matchers []MatcherSpec `json:"matchers,omitempty"`
	// Labels to set on the Silence when Condition is true.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations to set on the Silence when Condition is true.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Config struct holds all the configuration for the operator.
type Config struct {
	Address        string
	Authentication bool
	BearerToken    string
	TenantId       string

	// SilenceSelector is used to filter silences based on label selectors.
	// If nil, the controller will watch all silences.
	SilenceSelector labels.Selector
	// NamespaceSelector is used to restrict which namespaces the v2 controller watches.
	// If nil, the controller will watch all namespaces.
	NamespaceSelector labels.Selector

	// Tenancy configuration
	TenancyEnabled       bool
	TenancyLabelKey      string // Single label key to extract tenant from (e.g., "observability.giantswarm.io/tenant")
	TenancyDefaultTenant string
}

// parseSelector is a generic helper function that parses a selector string into a labels.Selector.
// Returns nil if the selector is empty.
func parseSelector(selectorString string) (labels.Selector, error) {
	if selectorString == "" {
		return nil, nil
	}

	parsedSelector, err := metav1.ParseToLabelSelector(selectorString)
	if err != nil {
		return nil, err
	}

	selector, err := metav1.LabelSelectorAsSelector(parsedSelector)
	if err != nil {
		return nil, err
	}

	return selector, nil
}

// ParseSilenceSelector parses a silence selector string into a labels.Selector.
// Returns nil if the selector is empty, which means no filtering will be applied.
func ParseSilenceSelector(silenceSelector string) (labels.Selector, error) {
	selector, err := parseSelector(silenceSelector)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse silence-selector string: %q", silenceSelector)
	}
	return selector, nil
}

// ParseNamespaceSelector parses a namespace selector string into a labels.Selector.
// Returns nil if the selector is empty, which means all namespaces will be watched.
func ParseNamespaceSelector(namespaceSelector string) (labels.Selector, error) {
	selector, err := parseSelector(namespaceSelector)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse namespace-selector string: %q", namespaceSelector)
	}
	return selector, nil
}
