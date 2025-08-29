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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MatchType defines the type of matching for alert matchers.
// +kubebuilder:validation:Enum==;!=;=~;!~
type MatchType string

const (
	// MatchEqual matches alerts where the label value exactly equals the matcher value
	MatchEqual MatchType = "="
	// MatchNotEqual matches alerts where the label value does not equal the matcher value
	MatchNotEqual MatchType = "!="
	// MatchRegexMatch matches alerts where the label value matches the regex pattern
	MatchRegexMatch MatchType = "=~"
	// MatchRegexNotMatch matches alerts where the label value does not match the regex pattern
	MatchRegexNotMatch MatchType = "!~"
)

// SilenceMatcher defines an alert matcher to be muted by the Silence.
type SilenceMatcher struct {
	// Name of the label to match.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Name string `json:"name"`
	// Value to match for the given label name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=1024
	Value string `json:"value"`
	// MatchType defines the type of matching to perform.
	// +kubebuilder:default="="
	// +optional
	MatchType MatchType `json:"matchType,omitempty"`
}

// SilenceSpec defines the desired state of Silence.
type SilenceSpec struct {
	// Matchers defines the alert matchers that this silence will apply to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Matchers []SilenceMatcher `json:"matchers"`

	// StartsAt defines when the silence becomes active. If not specified, defaults to the current time.
	// This field takes precedence over creation timestamp when both are available.
	// +optional
	StartsAt *metav1.Time `json:"startsAt,omitempty"`

	// EndsAt defines when the silence expires. If not specified, Duration is used to calculate the end time.
	// This field takes precedence over Duration and valid-until annotation when specified.
	// +optional
	EndsAt *metav1.Time `json:"endsAt,omitempty"`

	// Duration defines how long the silence should be active from StartsAt (or creation time if StartsAt is not set).
	// This field is ignored if EndsAt is specified. If neither EndsAt nor Duration is specified,
	// the valid-until annotation is used, or defaults to 100 years (same as v1alpha1 for backward compatibility).
	// Examples: "1h", "30m", "24h", "7d"
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`
}

// Silence is the Schema for the silences API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Starts At",type=date,JSONPath=`.spec.startsAt`
// +kubebuilder:printcolumn:name="Ends At",type=date,JSONPath=`.spec.endsAt`
// +kubebuilder:printcolumn:name="Duration",type=string,JSONPath=`.spec.duration`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="!(has(self.spec.endsAt) && has(self.spec.duration))",message="endsAt and duration are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.startsAt) || !has(self.spec.endsAt) || timestamp(self.spec.startsAt) < timestamp(self.spec.endsAt)",message="startsAt must be before endsAt"
type Silence struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SilenceSpec `json:"spec,omitempty"`
}

// SilenceList contains a list of Silence.
// +kubebuilder:object:root=true
type SilenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Silence `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Silence{}, &SilenceList{})
}
