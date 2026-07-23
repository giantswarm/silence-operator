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

// Package enforce implements namespace-scoped silence enforcement. When one or
// more rules are configured, a Silence created in a namespace matching a rule's
// namespaceSelector gets an authoritative "namespace=<namespace>" matcher (plus
// any custom matchers from the rule) injected into the Alertmanager silence, so
// that it can only ever mute alerts belonging to its own namespace.
package enforce

import (
	"os"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

// DefaultMatcherLabel is the matcher label used for the injected namespace
// matcher when the config does not override it.
const DefaultMatcherLabel = "namespace"

// FileConfig is the on-disk (YAML) representation of the enforcement config.
// JSON tags are used because it is parsed via sigs.k8s.io/yaml, which converts
// YAML to JSON so that Kubernetes types (e.g. metav1.LabelSelector) deserialize
// exactly as they would in a manifest.
type FileConfig struct {
	// MatcherLabel is the label used for the injected namespace matcher.
	// Defaults to "namespace" when empty.
	MatcherLabel string `json:"matcherLabel,omitempty"`
	// Rules are evaluated top-to-bottom; the first rule whose namespaceSelector
	// matches a namespace wins (first-match-wins).
	Rules []RuleConfig `json:"rules,omitempty"`
}

// RuleConfig is a single enforcement rule.
type RuleConfig struct {
	// NamespaceSelector selects the namespaces this rule applies to. An empty
	// selector matches ALL namespaces (standard Kubernetes semantics).
	NamespaceSelector metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	// Matchers are extra matchers added to silences in matching namespaces, in
	// addition to the namespace matcher.
	Matchers []MatcherConfig `json:"matchers,omitempty"`
}

// MatcherConfig is a custom matcher attached to a rule.
type MatcherConfig struct {
	Name string `json:"name"`
	// Value to match for the given label name.
	Value string `json:"value"`
	// MatchType is one of "=", "!=", "=~", "!~". Defaults to "=".
	MatchType string `json:"matchType,omitempty"`
}

// rule is a compiled enforcement rule held in memory.
type rule struct {
	selector labels.Selector
	matchers []alertmanager.Matcher
}

// Enforcer applies namespace-scoped enforcement to Alertmanager silences based
// on a set of compiled rules.
type Enforcer struct {
	matcherLabel string
	rules        []rule
	warnings     []string
}

// LoadFromFile reads and compiles the enforcement config from the given path.
// It fails fast on an unreadable file, malformed YAML, an invalid selector, or
// an invalid matcher so that misconfiguration surfaces at startup rather than
// at reconcile time.
func LoadFromFile(path string) (*Enforcer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read namespace enforcement config %q", path)
	}

	var fc FileConfig
	if err := yaml.UnmarshalStrict(data, &fc); err != nil {
		return nil, errors.Wrapf(err, "failed to parse namespace enforcement config %q", path)
	}

	return newEnforcer(fc)
}

// newEnforcer compiles a FileConfig into an Enforcer.
func newEnforcer(fc FileConfig) (*Enforcer, error) {
	e := &Enforcer{
		matcherLabel: fc.MatcherLabel,
	}
	if e.matcherLabel == "" {
		e.matcherLabel = DefaultMatcherLabel
	}

	for i, rc := range fc.Rules {
		selector, err := metav1.LabelSelectorAsSelector(&rc.NamespaceSelector)
		if err != nil {
			return nil, errors.Wrapf(err, "rule[%d]: invalid namespaceSelector", i)
		}

		// An empty selector matches everything; warn so an accidental empty
		// selector (which silently enforces on every namespace) is visible.
		if selector.Empty() {
			e.warnings = append(e.warnings, matchAllWarning(i))
		}

		var matchers []alertmanager.Matcher
		for j, mc := range rc.Matchers {
			if mc.Name == "" {
				return nil, errors.Errorf("rule[%d].matchers[%d]: name must not be empty", i, j)
			}
			m, err := alertmanager.NewMatcher(mc.MatchType, mc.Name, mc.Value)
			if err != nil {
				return nil, errors.Wrapf(err, "rule[%d].matchers[%d]", i, j)
			}
			matchers = append(matchers, m)
		}

		e.rules = append(e.rules, rule{selector: selector, matchers: matchers})
	}

	return e, nil
}

func matchAllWarning(i int) string {
	return errors.Errorf("namespace enforcement rule[%d] has an empty namespaceSelector and will be enforced on ALL namespaces", i).Error()
}

// Warnings returns non-fatal warnings collected while loading the config (e.g.
// rules that match all namespaces). Callers should log these at startup.
func (e *Enforcer) Warnings() []string {
	return e.warnings
}

// MatchersFor returns the enforced matchers for a namespace with the given name
// and labels, and whether any rule matched. The returned slice always leads
// with the authoritative namespace matcher, followed by the first matching
// rule's custom matchers (first-match-wins). It returns (nil, false) when no
// rule matches.
func (e *Enforcer) MatchersFor(namespaceName string, namespaceLabels map[string]string) ([]alertmanager.Matcher, bool) {
	set := labels.Set(namespaceLabels)
	for _, r := range e.rules {
		if !r.selector.Matches(set) {
			continue
		}

		// NewMatcher cannot error for a "=" match type.
		nsMatcher, _ := alertmanager.NewMatcher(alertmanager.MatchTypeEqual, e.matcherLabel, namespaceName)

		enforced := make([]alertmanager.Matcher, 0, 1+len(r.matchers))
		enforced = append(enforced, nsMatcher)
		enforced = append(enforced, r.matchers...)
		return enforced, true
	}
	return nil, false
}

// ApplyEnforcedMatchers overwrites, on the given silence, any matcher whose Name
// collides with an enforced matcher, then appends all enforced matchers. It
// returns the names of the matchers it replaced (deduplicated) so the caller can
// surface that a tenant-supplied matcher was overridden. Enforcement is
// authoritative: a tenant cannot override an enforced matcher.
func ApplyEnforcedMatchers(s *alertmanager.Silence, enforced []alertmanager.Matcher) []string {
	if len(enforced) == 0 {
		return nil
	}

	enforcedNames := make(map[string]struct{}, len(enforced))
	for _, m := range enforced {
		enforcedNames[m.Name] = struct{}{}
	}

	var replaced []string
	seen := make(map[string]struct{})
	kept := make([]alertmanager.Matcher, 0, len(s.Matchers)+len(enforced))
	for _, m := range s.Matchers {
		if _, collides := enforcedNames[m.Name]; collides {
			if _, dup := seen[m.Name]; !dup {
				seen[m.Name] = struct{}{}
				replaced = append(replaced, m.Name)
			}
			continue
		}
		kept = append(kept, m)
	}

	s.Matchers = append(kept, enforced...)
	return replaced
}
