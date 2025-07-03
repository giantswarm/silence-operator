package config

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Config struct holds all the configuration for the operator.
type Config struct {
	Address        string
	Authentication bool
	BearerToken    string
	TenantId       string
	// SilenceSelector is used to filter silences based on label selectors.
	SilenceSelector labels.Selector
	// NamespaceSelector is used to restrict which namespaces the v2 controller watches.
	// If nil, the controller will watch all namespaces.
	NamespaceSelector labels.Selector
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
