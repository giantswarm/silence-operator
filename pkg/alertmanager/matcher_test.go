package alertmanager

import "testing"

func TestNewMatcher(t *testing.T) {
	tests := []struct {
		name      string
		matchType string
		wantRegex bool
		wantEqual bool
		wantErr   bool
	}{
		{name: "default empty is equal", matchType: "", wantEqual: true},
		{name: "equal", matchType: "=", wantEqual: true},
		{name: "not equal", matchType: "!=", wantEqual: false},
		{name: "regex match", matchType: "=~", wantRegex: true, wantEqual: true},
		{name: "regex not match", matchType: "!~", wantRegex: true, wantEqual: false},
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
