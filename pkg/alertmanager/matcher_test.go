package alertmanager

import (
	"testing"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
)

func TestNewMatcher(t *testing.T) {
	tests := []struct {
		name      string
		matchType v1alpha2.MatchType
		wantRegex bool
		wantEqual bool
		wantErr   bool
	}{
		{name: "default empty is equal", matchType: "", wantEqual: true},
		{name: "equal", matchType: v1alpha2.MatchEqual, wantEqual: true},
		{name: "not equal", matchType: v1alpha2.MatchNotEqual, wantEqual: false},
		{name: "regex match", matchType: v1alpha2.MatchRegexMatch, wantRegex: true, wantEqual: true},
		{name: "regex not match", matchType: v1alpha2.MatchRegexNotMatch, wantRegex: true, wantEqual: false},
		{name: "invalid", matchType: "??", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.matchType, "foo", "bar")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for matchType %q", tt.matchType)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.IsRegex != tt.wantRegex || m.IsEqual != tt.wantEqual {
				t.Errorf("IsRegex=%v IsEqual=%v, want IsRegex=%v IsEqual=%v", m.IsRegex, m.IsEqual, tt.wantRegex, tt.wantEqual)
			}
			if m.Name != "foo" || m.Value != "bar" {
				t.Errorf("Name=%q Value=%q, want foo/bar", m.Name, m.Value)
			}
		})
	}
}
