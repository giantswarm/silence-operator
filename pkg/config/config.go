package config

import (
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
}
