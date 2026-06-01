/*
Copyright 2026.

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

package v1alpha2

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// These constants mirror pkg/alertmanager to avoid a downward dependency from
// the API package into pkg. The DDD/DRY pass should move them to a shared location.
const (
	validUntilAnnotation = "valid-until"
	dateOnlyLayout       = "2006-01-02"
)

// +kubebuilder:webhook:path=/validate-observability-giantswarm-io-v1alpha2-silence,mutating=false,failurePolicy=fail,sideEffects=None,groups=observability.giantswarm.io,resources=silences,verbs=create;update,versions=v1alpha2,name=vsilence.observability.giantswarm.io,admissionReviewVersions=v1

// SilenceValidator enforces business-rule constraints that cannot be expressed
// in the OpenAPI/CRD schema:
//
//   - No duplicate matchers (same name+value+matchType).
//   - Regex matchers (=~ / !~) must be valid Go regular expressions.
//   - The valid-until annotation, if present, must be parseable as RFC3339 or
//     date-only (YYYY-MM-DD).
//   - On CREATE: valid-until must not already be in the past.
type SilenceValidator struct{}

var _ admission.Validator[*Silence] = &SilenceValidator{}

// SetupWebhookWithManager registers the validating webhook with the manager.
func (v *SilenceValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &Silence{}).
		WithValidator(v).
		Complete()
}

func (v *SilenceValidator) ValidateCreate(ctx context.Context, silence *Silence) (admission.Warnings, error) {
	return validateSilence(silence, true)
}

func (v *SilenceValidator) ValidateUpdate(_ context.Context, _, newSilence *Silence) (admission.Warnings, error) {
	return validateSilence(newSilence, false)
}

func (v *SilenceValidator) ValidateDelete(_ context.Context, _ *Silence) (admission.Warnings, error) {
	return nil, nil
}

// validateSilence runs all validations and aggregates errors.
// isCreate controls whether the "valid-until not in the past" check applies.
func validateSilence(silence *Silence, isCreate bool) (admission.Warnings, error) {
	var errs []string
	errs = append(errs, validateNoDuplicateMatchers(silence.Spec.Matchers)...)
	errs = append(errs, validateRegexMatchers(silence.Spec.Matchers)...)
	errs = append(errs, validateValidUntil(silence, isCreate)...)
	if len(errs) > 0 {
		return nil, fmt.Errorf("Silence %q is invalid: %s", silence.Name, strings.Join(errs, "; "))
	}
	return nil, nil
}

// validateNoDuplicateMatchers rejects matchers that are identical in
// name, value, and matchType.
func validateNoDuplicateMatchers(matchers []SilenceMatcher) []string {
	var errs []string
	seen := make(map[string]int) // key → first index
	for i, m := range matchers {
		key := m.Name + "|" + m.Value + "|" + string(m.MatchType)
		if first, ok := seen[key]; ok {
			errs = append(errs, fmt.Sprintf(
				"spec.matchers[%d] duplicates spec.matchers[%d] (name=%q value=%q matchType=%q)",
				i, first, m.Name, m.Value, m.MatchType,
			))
		} else {
			seen[key] = i
		}
	}
	return errs
}

// validateRegexMatchers checks that =~ and !~ matchers contain a valid Go regex.
func validateRegexMatchers(matchers []SilenceMatcher) []string {
	var errs []string
	for i, m := range matchers {
		if m.MatchType == MatchRegexMatch || m.MatchType == MatchRegexNotMatch {
			if _, err := regexp.Compile(m.Value); err != nil {
				errs = append(errs, fmt.Sprintf(
					"spec.matchers[%d]: matchType %q requires a valid regular expression, %q does not compile: %v",
					i, m.MatchType, m.Value, err,
				))
			}
		}
	}
	return errs
}

// validateValidUntil checks that the valid-until annotation is parseable, and
// on CREATE that the resulting expiry time is not already in the past.
func validateValidUntil(silence *Silence, isCreate bool) []string {
	val, ok := silence.GetAnnotations()[validUntilAnnotation]
	if !ok || val == "" {
		return nil
	}

	var expiry time.Time

	// Try RFC3339 first (e.g. 2026-12-31T08:00:00Z).
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		expiry = t
	} else {
		// Fall back to date-only (e.g. 2026-12-31).  The controller shifts
		// date-only values to 08:00 UTC when computing the actual Alertmanager
		// end-time, so we apply the same shift here for consistency.
		t, dateErr := time.Parse(dateOnlyLayout, val)
		if dateErr != nil {
			return []string{fmt.Sprintf(
				"annotation %q: %q is not a valid date; accepted formats are RFC3339 (e.g. 2026-12-31T08:00:00Z) or date-only (e.g. 2026-12-31)",
				validUntilAnnotation, val,
			)}
		}
		expiry = time.Date(t.Year(), t.Month(), t.Day(), 8, 0, 0, 0, time.UTC)
	}

	if isCreate && expiry.Before(time.Now()) {
		return []string{fmt.Sprintf(
			"annotation %q: expiry time %q is already in the past",
			validUntilAnnotation, val,
		)}
	}
	return nil
}
