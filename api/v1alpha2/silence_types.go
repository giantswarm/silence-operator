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
}

// Silence is the Schema for the silences API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
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
