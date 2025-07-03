/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tenancy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/silence-operator/pkg/config"
)

// Helper provides common tenancy functionality for both v1alpha1 and v1alpha2 controllers
type Helper struct {
	config config.Config
}

// NewHelper creates a new tenancy helper
func NewHelper(cfg config.Config) *Helper {
	return &Helper{
		config: cfg,
	}
}

// ExtractTenant extracts tenant information from a resource's labels
func (h *Helper) ExtractTenant(obj metav1.Object) string {
	if !h.config.TenancyEnabled {
		// If tenancy is disabled, return empty string (no tenant)
		return ""
	}

	if h.config.TenancyLabelKey == "" {
		// If no label key is configured, return default tenant
		return h.config.TenancyDefaultTenant
	}

	// Extract tenant from the configured label key
	if obj.GetLabels() != nil {
		if tenant, exists := obj.GetLabels()[h.config.TenancyLabelKey]; exists && tenant != "" {
			return tenant
		}
	}

	// Fall back to default tenant
	return h.config.TenancyDefaultTenant
}
