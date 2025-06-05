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

// ParseSilenceSelector parses a silence selector string into a labels.Selector.
// Returns nil if the selector is empty, which means no filtering will be applied.
func ParseSilenceSelector(silenceSelector string) (labels.Selector, error) {
	if silenceSelector == "" {
		return nil, nil
	}

	parsedSelector, err := metav1.ParseToLabelSelector(silenceSelector)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse silence-selector string: %q", silenceSelector)
	}

	selector, err := metav1.LabelSelectorAsSelector(parsedSelector)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert silence-selector to labels.Selector")
	}

	return selector, nil
}

// ParseNamespaceSelector parses a namespace selector string into a labels.Selector.
// Returns nil if the selector is empty, which means all namespaces will be watched.
func ParseNamespaceSelector(namespaceSelector string) (labels.Selector, error) {
	if namespaceSelector == "" {
		return nil, nil
	}

	parsedSelector, err := metav1.ParseToLabelSelector(namespaceSelector)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse namespace-selector string: %q", namespaceSelector)
	}

	selector, err := metav1.LabelSelectorAsSelector(parsedSelector)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert namespace-selector to labels.Selector")
	}

	return selector, nil
}
