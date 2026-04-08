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
	"encoding/json"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/giantswarm/silence-operator/pkg/config"
)

// compiledCELRule holds a CELRule together with its pre-compiled CEL program.
type compiledCELRule struct {
	rule    config.CELRule
	program cel.Program // nil when Condition is empty (always matches)
}

// +kubebuilder:webhook:path=/mutate-observability-giantswarm-io-v1alpha2-silence,mutating=true,failurePolicy=fail,sideEffects=None,groups=observability.giantswarm.io,resources=silences,verbs=create;update,versions=v1alpha2,name=msilence.observability.giantswarm.io,admissionReviewVersions=v1

// SilenceDefaulter implements admission.Defaulter[*Silence] and applies CEL-based
// mutation rules to every Silence on CREATE/UPDATE.
//
// Rules with an empty Condition always match (unconditional injection).
// Rules with a non-empty Condition are evaluated against the incoming object.
// Matcher injection is idempotent — already-present matchers are not duplicated.
type SilenceDefaulter struct {
	celRules []compiledCELRule
}

// Ensure SilenceDefaulter implements admission.Defaulter[*Silence].
var _ admission.Defaulter[*Silence] = &SilenceDefaulter{}

// NewSilenceDefaulter creates a SilenceDefaulter and pre-compiles all CEL expressions.
// Returns an error if any CEL condition fails to compile.
func NewSilenceDefaulter(cfg config.WebhookConfig) (*SilenceDefaulter, error) {
	if len(cfg.CELRules) == 0 {
		return &SilenceDefaulter{}, nil
	}

	// Build a CEL environment with a single "object" variable holding the
	// Silence as a dynamic map (JSON-decoded). DynType is intentional: it
	// lets callers navigate arbitrary fields without a schema.
	env, err := cel.NewEnv(
		cel.Variable("object", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create CEL environment")
	}

	rules := make([]compiledCELRule, 0, len(cfg.CELRules))
	for _, rule := range cfg.CELRules {
		cr := compiledCELRule{rule: rule}
		if rule.Condition != "" {
			ast, issues := env.Compile(rule.Condition)
			if issues != nil && issues.Err() != nil {
				return nil, fmt.Errorf("CEL rule %q: compilation error: %w", rule.Name, issues.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				return nil, fmt.Errorf("CEL rule %q: program error: %w", rule.Name, err)
			}
			cr.program = prg
		}
		rules = append(rules, cr)
	}

	return &SilenceDefaulter{celRules: rules}, nil
}

// SetupWebhookWithManager registers the defaulting webhook with the controller manager.
// The path /mutate-observability-giantswarm-io-v1alpha2-silence is registered automatically.
func (d *SilenceDefaulter) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &Silence{}).
		WithDefaulter(d).
		Complete()
}

// Default is called by the webhook server on every CREATE/UPDATE admission request.
func (d *SilenceDefaulter) Default(ctx context.Context, silence *Silence) error {
	if len(d.celRules) == 0 {
		return nil
	}

	objMap, err := silenceToMap(silence)
	if err != nil {
		return errors.Wrap(err, "failed to convert Silence to map for CEL evaluation")
	}

	for _, cr := range d.celRules {
		matched, err := evaluateCELRule(cr, objMap)
		if err != nil {
			return fmt.Errorf("CEL rule %q evaluation failed: %w", cr.rule.Name, err)
		}
		if !matched {
			continue
		}

		for _, ms := range cr.rule.Matchers {
			if !hasMatcher(silence.Spec.Matchers, ms) {
				silence.Spec.Matchers = append(silence.Spec.Matchers, SilenceMatcher{
					Name:      ms.Name,
					Value:     ms.Value,
					MatchType: MatchType(ms.MatchType),
				})
			}
		}

		for k, v := range cr.rule.Labels {
			if silence.Labels == nil {
				silence.Labels = map[string]string{}
			}
			silence.Labels[k] = v
		}

		for k, v := range cr.rule.Annotations {
			if silence.Annotations == nil {
				silence.Annotations = map[string]string{}
			}
			silence.Annotations[k] = v
		}
	}

	return nil
}

// hasMatcher returns true when matchers already contains an entry identical to ms,
// making injection idempotent.
func hasMatcher(matchers []SilenceMatcher, ms config.MatcherSpec) bool {
	for _, m := range matchers {
		if m.Name == ms.Name && m.Value == ms.Value && string(m.MatchType) == ms.MatchType {
			return true
		}
	}
	return false
}

// evaluateCELRule evaluates the rule's condition against objMap.
// A nil program (empty Condition) always matches.
func evaluateCELRule(cr compiledCELRule, objMap map[string]interface{}) (bool, error) {
	if cr.program == nil {
		return true, nil
	}
	out, _, err := cr.program.Eval(map[string]interface{}{"object": objMap})
	if err != nil {
		return false, err
	}
	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression must return bool, got %T", out.Value())
	}
	return result, nil
}

// silenceToMap converts a Silence to a map[string]interface{} via JSON round-trip.
// This produces the shape available in CEL expressions as the "object" variable:
//
//	object.metadata.namespace
//	object.metadata.labels["team"]
//	"team" in object.metadata.labels
//	object.spec.matchers
func silenceToMap(s *Silence) (map[string]interface{}, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
