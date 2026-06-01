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
)

var v = &SilenceValidator{}

func silence(matchers ...SilenceMatcher) *Silence {
	return &Silence{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec:       SilenceSpec{Matchers: matchers},
	}
}

func matcher(name, value string, mt MatchType) SilenceMatcher {
	return SilenceMatcher{Name: name, Value: value, MatchType: mt}
}

func withAnnotation(s *Silence, key, val string) *Silence {
	if s.Annotations == nil {
		s.Annotations = map[string]string{}
	}
	s.Annotations[key] = val
	return s
}

// --- ValidateCreate: valid input ---

func TestValidateCreate_Valid(t *testing.T) {
	s := silence(matcher("alertname", "HighCPU", MatchEqual))
	_, err := v.ValidateCreate(context.Background(), s)
	require.NoError(t, err)
}

// --- Duplicate matchers ---

func TestValidateCreate_DuplicateMatcher(t *testing.T) {
	s := silence(
		matcher("alertname", "HighCPU", MatchEqual),
		matcher("alertname", "HighCPU", MatchEqual),
	)
	_, err := v.ValidateCreate(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicates")
	assert.Contains(t, err.Error(), "spec.matchers[1]")
}

func TestValidateCreate_SameNameDifferentValue_OK(t *testing.T) {
	s := silence(
		matcher("alertname", "HighCPU", MatchEqual),
		matcher("alertname", "LowMemory", MatchEqual),
	)
	_, err := v.ValidateCreate(context.Background(), s)
	require.NoError(t, err)
}

func TestValidateCreate_SameNameValueDifferentMatchType_OK(t *testing.T) {
	s := silence(
		matcher("alertname", "Heartbeat", MatchEqual),
		matcher("alertname", "Heartbeat", MatchNotEqual),
	)
	_, err := v.ValidateCreate(context.Background(), s)
	require.NoError(t, err)
}

func TestValidateCreate_MultipleDuplicates_AllReported(t *testing.T) {
	s := silence(
		matcher("a", "1", MatchEqual),
		matcher("b", "2", MatchEqual),
		matcher("a", "1", MatchEqual), // dup of [0]
		matcher("b", "2", MatchEqual), // dup of [1]
	)
	_, err := v.ValidateCreate(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.matchers[2]")
	assert.Contains(t, err.Error(), "spec.matchers[3]")
}

// --- Regex matchers ---

func TestValidateCreate_ValidRegex(t *testing.T) {
	s := silence(matcher("alertname", "High.*", MatchRegexMatch))
	_, err := v.ValidateCreate(context.Background(), s)
	require.NoError(t, err)
}

func TestValidateCreate_InvalidRegex(t *testing.T) {
	s := silence(matcher("alertname", "[invalid", MatchRegexMatch))
	_, err := v.ValidateCreate(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not compile")
}

func TestValidateCreate_InvalidNegativeRegex(t *testing.T) {
	s := silence(matcher("alertname", "(unclosed", MatchRegexNotMatch))
	_, err := v.ValidateCreate(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not compile")
}

func TestValidateCreate_EqualMatchType_RegexNotValidated(t *testing.T) {
	// = and != matchers treat value as a literal string, not a regex.
	s := silence(matcher("alertname", "[not-validated-as-regex", MatchEqual))
	_, err := v.ValidateCreate(context.Background(), s)
	require.NoError(t, err)
}

// --- valid-until annotation ---

func TestValidateCreate_ValidUntil_RFC3339_Future(t *testing.T) {
	s := withAnnotation(silence(matcher("a", "b", MatchEqual)), validUntilAnnotation, "2099-12-31T08:00:00Z")
	_, err := v.ValidateCreate(context.Background(), s)
	require.NoError(t, err)
}

func TestValidateCreate_ValidUntil_DateOnly_Future(t *testing.T) {
	s := withAnnotation(silence(matcher("a", "b", MatchEqual)), validUntilAnnotation, "2099-12-31")
	_, err := v.ValidateCreate(context.Background(), s)
	require.NoError(t, err)
}

func TestValidateCreate_ValidUntil_RFC3339_Past(t *testing.T) {
	s := withAnnotation(silence(matcher("a", "b", MatchEqual)), validUntilAnnotation, "2020-01-01T00:00:00Z")
	_, err := v.ValidateCreate(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in the past")
}

func TestValidateCreate_ValidUntil_DateOnly_Past(t *testing.T) {
	s := withAnnotation(silence(matcher("a", "b", MatchEqual)), validUntilAnnotation, "2020-01-01")
	_, err := v.ValidateCreate(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in the past")
}

func TestValidateCreate_ValidUntil_InvalidFormat(t *testing.T) {
	s := withAnnotation(silence(matcher("a", "b", MatchEqual)), validUntilAnnotation, "not-a-date")
	_, err := v.ValidateCreate(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid date")
}

func TestValidateCreate_ValidUntil_Absent_OK(t *testing.T) {
	s := silence(matcher("a", "b", MatchEqual))
	_, err := v.ValidateCreate(context.Background(), s)
	require.NoError(t, err)
}

// --- UPDATE does not apply the "in the past" check ---

func TestValidateUpdate_ValidUntil_Past_Allowed(t *testing.T) {
	// Updating a Silence that already has an expired valid-until should succeed:
	// the controller will expire it naturally; the webhook must not block updates.
	old := withAnnotation(silence(matcher("a", "b", MatchEqual)), validUntilAnnotation, "2020-01-01T00:00:00Z")
	updated := withAnnotation(silence(matcher("a", "b", MatchEqual), matcher("c", "d", MatchEqual)), validUntilAnnotation, "2020-01-01T00:00:00Z")
	_, err := v.ValidateUpdate(context.Background(), old, updated)
	require.NoError(t, err)
}

// --- DELETE is always allowed ---

func TestValidateDelete_AlwaysAllowed(t *testing.T) {
	s := silence(matcher("a", "b", MatchEqual))
	_, err := v.ValidateDelete(context.Background(), s)
	require.NoError(t, err)
}

// --- Multiple errors aggregated ---

func TestValidateCreate_MultipleErrors_AllReported(t *testing.T) {
	s := withAnnotation(
		silence(
			matcher("alertname", "[bad-regex", MatchRegexMatch),
			matcher("alertname", "[bad-regex", MatchRegexMatch), // also a duplicate
		),
		validUntilAnnotation, "not-a-date",
	)
	_, err := v.ValidateCreate(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicates")
	assert.Contains(t, err.Error(), "does not compile")
	assert.Contains(t, err.Error(), "not a valid date")
}
