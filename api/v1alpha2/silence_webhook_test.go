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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/silence-operator/pkg/config"
)

func newDefaulter(t *testing.T, cfg config.WebhookConfig) *SilenceDefaulter {
	t.Helper()
	d, err := NewSilenceDefaulter(cfg)
	require.NoError(t, err)
	return d
}

func matcherSpec(name, value, mt string) config.MatcherSpec {
	return config.MatcherSpec{Name: name, Value: value, MatchType: mt}
}

// --- No-op when no rules are configured ---

func TestNoRules_NoChange(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{})
	silence := &Silence{
		Spec: SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "Foo", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assert.Len(t, silence.Spec.Matchers, 1)
}

// --- Empty condition (always-match) ---

func TestEmptyCondition_AlwaysApplied(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{Name: "always", Condition: "", Matchers: []config.MatcherSpec{matcherSpec("alertname", "Heartbeat", "!=")}},
		},
	})
	silence := &Silence{
		Spec: SilenceSpec{Matchers: []SilenceMatcher{{Name: "severity", Value: "critical", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assertHasMatcher(t, silence, "alertname", "Heartbeat", MatchNotEqual)
}

func TestEmptyCondition_Idempotent(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{Name: "always", Condition: "", Matchers: []config.MatcherSpec{matcherSpec("alertname", "Heartbeat", "!=")}},
		},
	})
	silence := &Silence{
		Spec: SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "Heartbeat", MatchType: MatchNotEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	require.NoError(t, d.Default(context.Background(), silence)) // second call simulates UPDATE
	assert.Len(t, silence.Spec.Matchers, 1)
}

// --- Namespace condition ---

func TestNamespaceCondition_Matches(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{
				Name:      "production-only",
				Condition: `object.metadata.namespace == "production"`,
				Matchers:  []config.MatcherSpec{matcherSpec("alertname", "Heartbeat", "!=")},
			},
		},
	})
	silence := &Silence{
		ObjectMeta: metav1.ObjectMeta{Namespace: "production"},
		Spec:       SilenceSpec{Matchers: []SilenceMatcher{{Name: "severity", Value: "critical", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assertHasMatcher(t, silence, "alertname", "Heartbeat", MatchNotEqual)
}

func TestNamespaceCondition_NoMatch(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{
				Name:      "production-only",
				Condition: `object.metadata.namespace == "production"`,
				Matchers:  []config.MatcherSpec{matcherSpec("alertname", "Heartbeat", "!=")},
			},
		},
	})
	silence := &Silence{
		ObjectMeta: metav1.ObjectMeta{Namespace: "staging"},
		Spec:       SilenceSpec{Matchers: []SilenceMatcher{{Name: "severity", Value: "critical", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assert.Len(t, silence.Spec.Matchers, 1)
}

// --- Label condition ---

func TestLabelCondition_Matches(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{
				Name:      "team-label",
				Condition: `"team" in object.metadata.labels`,
				Labels:    map[string]string{"observability.giantswarm.io/tagged": "true"},
			},
		},
	})
	silence := &Silence{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"team": "atlas"}},
		Spec:       SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "X", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assert.Equal(t, "true", silence.Labels["observability.giantswarm.io/tagged"])
}

func TestLabelCondition_NoMatch(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{
				Name:      "team-label",
				Condition: `"team" in object.metadata.labels`,
				Labels:    map[string]string{"observability.giantswarm.io/tagged": "true"},
			},
		},
	})
	silence := &Silence{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"other": "value"}},
		Spec:       SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "X", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assert.Empty(t, silence.Labels["observability.giantswarm.io/tagged"])
}

// --- Annotation injection ---

func TestAnnotationInjection(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{Name: "annotate", Condition: "", Annotations: map[string]string{"injected-by": "silence-operator"}},
		},
	})
	silence := &Silence{
		Spec: SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "X", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assert.Equal(t, "silence-operator", silence.Annotations["injected-by"])
}

// --- Error cases ---

func TestInvalidCELCondition_ReturnsError(t *testing.T) {
	_, err := NewSilenceDefaulter(config.WebhookConfig{
		CELRules: []config.CELRule{
			{Name: "bad", Condition: "this is not valid CEL )("},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad")
}

func TestNonBoolCELCondition_ReturnsError(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{{Name: "returns-string", Condition: `"hello"`}},
	})
	silence := &Silence{
		Spec: SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "X", MatchType: MatchEqual}}},
	}
	assert.Error(t, d.Default(context.Background(), silence))
}

// --- Label and annotation overwrite behaviour ---
//
// Labels and annotations are always overwritten (last-write-wins across rules).
// This is intentional and different from matchers, which are deduplicated.

func TestLabelOverwrite(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{Name: "enforce-team", Condition: "", Labels: map[string]string{"team": "backend"}},
		},
	})
	silence := &Silence{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"team": "frontend"}},
		Spec:       SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "X", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assert.Equal(t, "backend", silence.Labels["team"], "rule should overwrite existing label value")
}

func TestAnnotationOverwrite(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{Name: "stamp", Condition: "", Annotations: map[string]string{"managed-by": "webhook"}},
		},
	})
	silence := &Silence{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"managed-by": "user"}},
		Spec:       SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "X", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assert.Equal(t, "webhook", silence.Annotations["managed-by"], "rule should overwrite existing annotation value")
}

// --- Regex and negated-regex matcher injection ---

func TestRegexMatcherInjection(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{Name: "regex-rule", Condition: "", Matchers: []config.MatcherSpec{matcherSpec("alertname", "High.*", "=~")}},
		},
	})
	silence := &Silence{
		Spec: SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "HighCPU", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assertHasMatcher(t, silence, "alertname", "High.*", MatchType("=~"))
	assert.Len(t, silence.Spec.Matchers, 2)
}

// --- Runtime CEL errors ---

// A condition that dereferences a field absent from the object (e.g. labels when
// ObjectMeta.Labels is nil) should propagate as an error, not silently skip the rule.
// Users should use the CEL has() macro to guard optional fields:
//
//	has(object.metadata.labels) && "team" in object.metadata.labels
func TestCELRuntimeError_MissingField(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{
				Name:      "requires-labels",
				Condition: `"team" in object.metadata.labels`,
				Matchers:  []config.MatcherSpec{matcherSpec("alertname", "Heartbeat", "!=")},
			},
		},
	})
	// Silence with no labels — object.metadata.labels is absent from the JSON map.
	silence := &Silence{
		Spec: SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "X", MatchType: MatchEqual}}},
	}
	assert.Error(t, d.Default(context.Background(), silence), "expected error when CEL accesses absent labels map")
}

func TestCELRuntimeError_SafeGuardWithHas(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{
				Name:      "safe-label-check",
				Condition: `has(object.metadata.labels) && "team" in object.metadata.labels`,
				Matchers:  []config.MatcherSpec{matcherSpec("alertname", "Heartbeat", "!=")},
			},
		},
	})
	// Silence with no labels — has() guard prevents the runtime error.
	silence := &Silence{
		Spec: SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "X", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	assert.Len(t, silence.Spec.Matchers, 1, "rule should not fire when labels are absent")
}

// --- Multi-rule order ---

// Rules are applied in the order they are declared in celRules.
// Each rule's matchers are appended after the previous rule's injections.
func TestMultipleRules_AppliedInOrder(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{Name: "first", Condition: "", Matchers: []config.MatcherSpec{matcherSpec("alertname", "Heartbeat", "!=")}},
			{Name: "second", Condition: "", Matchers: []config.MatcherSpec{matcherSpec("all_pipelines", "true", "!=")}},
		},
	})
	silence := &Silence{
		Spec: SilenceSpec{Matchers: []SilenceMatcher{{Name: "alertname", Value: "X", MatchType: MatchEqual}}},
	}
	require.NoError(t, d.Default(context.Background(), silence))
	require.Len(t, silence.Spec.Matchers, 3)
	assert.Equal(t, "Heartbeat", silence.Spec.Matchers[1].Value, "first rule's matcher should be at index 1")
	assert.Equal(t, "true", silence.Spec.Matchers[2].Value, "second rule's matcher should be at index 2")
}

// --- Kyverno parity ---
//
// Verifies that the webhook can replicate the behaviour of the Kyverno policy in
// giantswarm/kyverno-policies-observability/policies/observability/Silence.yaml
// using two CEL rules with empty conditions (always-match).

func TestKyvernoParity(t *testing.T) {
	d := newDefaulter(t, config.WebhookConfig{
		CELRules: []config.CELRule{
			{Name: "exclude-heartbeat", Condition: "", Matchers: []config.MatcherSpec{matcherSpec("alertname", "Heartbeat", "!=")}},
			{Name: "exclude-all-pipelines", Condition: "", Matchers: []config.MatcherSpec{matcherSpec("all_pipelines", "true", "!=")}},
		},
	})

	silence := &Silence{
		Spec: SilenceSpec{
			Matchers: []SilenceMatcher{
				{Name: "alertname", Value: "HighCPU", MatchType: MatchEqual},
				{Name: "severity", Value: "warning", MatchType: MatchEqual},
			},
		},
	}
	require.NoError(t, d.Default(context.Background(), silence))

	assert.Len(t, silence.Spec.Matchers, 4)
	assertHasMatcher(t, silence, "alertname", "Heartbeat", MatchNotEqual)
	assertHasMatcher(t, silence, "all_pipelines", "true", MatchNotEqual)
	// User matchers must be preserved
	assertHasMatcher(t, silence, "alertname", "HighCPU", MatchEqual)
	assertHasMatcher(t, silence, "severity", "warning", MatchEqual)
}

// assertHasMatcher fails the test if the Silence doesn't contain the given matcher.
func assertHasMatcher(t *testing.T, s *Silence, name, value string, mt MatchType) {
	t.Helper()
	for _, m := range s.Spec.Matchers {
		if m.Name == name && m.Value == value && m.MatchType == mt {
			return
		}
	}
	t.Errorf("expected matcher {name:%q value:%q matchType:%q} not found in %+v", name, value, mt, s.Spec.Matchers)
}
