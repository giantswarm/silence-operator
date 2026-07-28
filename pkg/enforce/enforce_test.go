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

package enforce

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

func labelSelector(matchLabels map[string]string) metav1.LabelSelector {
	return metav1.LabelSelector{MatchLabels: matchLabels}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func TestLoadFromFile_Valid(t *testing.T) {
	path := writeConfig(t, `
namespaceMatcherLabel: namespace
rules:
  - namespaceSelector:
      matchLabels:
        tenant-isolation: "enabled"
  - namespaceSelector:
      matchExpressions:
        - key: team
          operator: In
          values: [platform]
    matchers:
      - name: cluster_id
        value: prod
        matchType: "="
`)

	e, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.namespaceMatcherLabel != "namespace" {
		t.Errorf("namespaceMatcherLabel = %q, want %q", e.namespaceMatcherLabel, "namespace")
	}
	if len(e.rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(e.rules))
	}
	if len(e.Warnings()) != 0 {
		t.Errorf("Warnings() = %v, want none", e.Warnings())
	}
}

func TestLoadFromFile_EmptyMatcherLabelStaysEmpty(t *testing.T) {
	path := writeConfig(t, `
rules:
  - namespaceSelector:
      matchLabels:
        a: "b"
    matchers:
      - name: cluster_id
        value: prod
`)
	e, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.namespaceMatcherLabel != "" {
		t.Errorf("namespaceMatcherLabel = %q, want empty (no defaulting)", e.namespaceMatcherLabel)
	}
}

func TestLoadFromFile_NoopRuleWarns(t *testing.T) {
	// A rule with no matchers and no namespaceMatcherLabel injects nothing.
	path := writeConfig(t, `
rules:
  - namespaceSelector:
      matchLabels:
        a: "b"
`)
	e, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(e.Warnings()) != 1 {
		t.Fatalf("Warnings() = %v, want exactly 1 (no-op rule)", e.Warnings())
	}
}

func TestLoadFromFile_EmptySelectorWarns(t *testing.T) {
	path := writeConfig(t, `
rules:
  - matchers:
      - name: cluster_id
        value: prod
`)
	e, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(e.Warnings()) != 1 {
		t.Fatalf("Warnings() = %v, want exactly 1", e.Warnings())
	}
}

func TestLoadFromFile_InvalidMatchType(t *testing.T) {
	path := writeConfig(t, `
rules:
  - namespaceSelector:
      matchLabels:
        a: "b"
    matchers:
      - name: foo
        value: bar
        matchType: "??"
`)
	if _, err := LoadFromFile(path); err == nil {
		t.Fatal("expected error for invalid matchType, got nil")
	}
}

func TestLoadFromFile_EmptyMatcherName(t *testing.T) {
	path := writeConfig(t, `
rules:
  - namespaceSelector:
      matchLabels:
        a: "b"
    matchers:
      - value: bar
`)
	if _, err := LoadFromFile(path); err == nil {
		t.Fatal("expected error for empty matcher name, got nil")
	}
}

func TestLoadFromFile_UnknownFieldRejected(t *testing.T) {
	path := writeConfig(t, `
rules:
  - namespaceSelector:
      matchLabels:
        a: "b"
    unknownField: true
`)
	if _, err := LoadFromFile(path); err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestMatchersFor(t *testing.T) {
	e, err := newEnforcer(FileConfig{
		NamespaceMatcherLabel: "namespace",
		Rules: []RuleConfig{
			{
				NamespaceSelector: labelSelector(map[string]string{"tier": "gold"}),
				Matchers: []MatcherConfig{
					{Name: "cluster_id", Value: "prod", MatchType: "="},
				},
			},
			{
				NamespaceSelector: labelSelector(map[string]string{"tier": "silver"}),
			},
		},
	})
	if err != nil {
		t.Fatalf("newEnforcer: %v", err)
	}

	tests := []struct {
		name       string
		nsName     string
		nsLabels   map[string]string
		wantMatch  bool
		wantValues []alertmanager.Matcher
	}{
		{
			name:      "matches first rule with custom matcher",
			nsName:    "team-a",
			nsLabels:  map[string]string{"tier": "gold"},
			wantMatch: true,
			wantValues: []alertmanager.Matcher{
				{Name: "namespace", Value: "team-a", IsEqual: true},
				{Name: "cluster_id", Value: "prod", IsEqual: true},
			},
		},
		{
			name:      "matches second rule, namespace matcher only",
			nsName:    "team-b",
			nsLabels:  map[string]string{"tier": "silver"},
			wantMatch: true,
			wantValues: []alertmanager.Matcher{
				{Name: "namespace", Value: "team-b", IsEqual: true},
			},
		},
		{
			name:      "no rule matches",
			nsName:    "team-c",
			nsLabels:  map[string]string{"tier": "bronze"},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, matched := e.MatchersFor(tt.nsName, tt.nsLabels)
			if matched != tt.wantMatch {
				t.Fatalf("matched = %v, want %v", matched, tt.wantMatch)
			}
			if !matched {
				return
			}
			if !reflect.DeepEqual(got, tt.wantValues) {
				t.Errorf("matchers = %+v, want %+v", got, tt.wantValues)
			}
		})
	}
}

func TestMatchersFor_FirstMatchWins(t *testing.T) {
	// A namespace matching both rules should get the first rule's matchers.
	e, err := newEnforcer(FileConfig{
		Rules: []RuleConfig{
			{
				NamespaceSelector: labelSelector(map[string]string{"a": "1"}),
				Matchers:          []MatcherConfig{{Name: "rule", Value: "first"}},
			},
			{
				NamespaceSelector: labelSelector(map[string]string{"b": "2"}),
				Matchers:          []MatcherConfig{{Name: "rule", Value: "second"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("newEnforcer: %v", err)
	}

	got, matched := e.MatchersFor("ns", map[string]string{"a": "1", "b": "2"})
	if !matched {
		t.Fatal("expected match")
	}
	// No namespaceMatcherLabel configured, so only the first rule's matcher.
	if len(got) != 1 || got[0].Value != "first" {
		t.Errorf("got %+v, want only the first rule's matcher", got)
	}
}

func TestMatchersFor_NoNamespaceMatcherWhenLabelEmpty(t *testing.T) {
	// Theo's scenario: empty namespaceMatcherLabel means no namespace matcher is
	// injected, only the matching rule's own matchers.
	e, err := newEnforcer(FileConfig{
		Rules: []RuleConfig{
			{
				NamespaceSelector: labelSelector(map[string]string{"team": "platform"}),
				Matchers:          []MatcherConfig{{Name: "cluster_id", Value: "prod", MatchType: "="}},
			},
		},
	})
	if err != nil {
		t.Fatalf("newEnforcer: %v", err)
	}

	got, matched := e.MatchersFor("my-namespace", map[string]string{"team": "platform"})
	if !matched {
		t.Fatal("expected match")
	}
	want := []alertmanager.Matcher{{Name: "cluster_id", Value: "prod", IsEqual: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matchers = %+v, want %+v (no namespace matcher)", got, want)
	}
}

func TestApplyEnforcedMatchers(t *testing.T) {
	tests := []struct {
		name         string
		existing     []alertmanager.Matcher
		enforced     []alertmanager.Matcher
		wantMatchers []alertmanager.Matcher
		wantReplaced []string
	}{
		{
			name:     "appends when no collision",
			existing: []alertmanager.Matcher{{Name: "alertname", Value: "Foo", IsEqual: true}},
			enforced: []alertmanager.Matcher{{Name: "namespace", Value: "a", IsEqual: true}},
			wantMatchers: []alertmanager.Matcher{
				{Name: "alertname", Value: "Foo", IsEqual: true},
				{Name: "namespace", Value: "a", IsEqual: true},
			},
			wantReplaced: nil,
		},
		{
			name: "overrides colliding user matcher",
			existing: []alertmanager.Matcher{
				{Name: "alertname", Value: "Foo", IsEqual: true},
				{Name: "namespace", Value: "other", IsRegex: true, IsEqual: true},
			},
			enforced: []alertmanager.Matcher{{Name: "namespace", Value: "a", IsEqual: true}},
			wantMatchers: []alertmanager.Matcher{
				{Name: "alertname", Value: "Foo", IsEqual: true},
				{Name: "namespace", Value: "a", IsEqual: true},
			},
			wantReplaced: []string{"namespace"},
		},
		{
			name: "overrides custom matcher too",
			existing: []alertmanager.Matcher{
				{Name: "cluster_id", Value: "staging", IsEqual: true},
			},
			enforced: []alertmanager.Matcher{
				{Name: "namespace", Value: "a", IsEqual: true},
				{Name: "cluster_id", Value: "prod", IsEqual: true},
			},
			wantMatchers: []alertmanager.Matcher{
				{Name: "namespace", Value: "a", IsEqual: true},
				{Name: "cluster_id", Value: "prod", IsEqual: true},
			},
			wantReplaced: []string{"cluster_id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &alertmanager.Silence{Matchers: tt.existing}
			replaced := ApplyEnforcedMatchers(s, tt.enforced)
			if !reflect.DeepEqual(s.Matchers, tt.wantMatchers) {
				t.Errorf("matchers = %+v, want %+v", s.Matchers, tt.wantMatchers)
			}
			if !reflect.DeepEqual(replaced, tt.wantReplaced) {
				t.Errorf("replaced = %v, want %v", replaced, tt.wantReplaced)
			}
		})
	}
}
