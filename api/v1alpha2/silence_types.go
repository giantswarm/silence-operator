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

// SilenceMatcher defines an alert matcher to be muted by the Silence.
type SilenceMatcher struct {
	// Name of the label to match.
	Name string `json:"name"`
	// Value to match for the given label name.
	Value string `json:"value"`
	// IsRegex defines whether the provided value should be interpreted as a regular expression.
	// +optional
	IsRegex bool `json:"isRegex,omitempty"`
	// IsEqual defines whether the provided value should match or not match the actual label value.
	// +optional
	IsEqual *bool `json:"isEqual,omitempty"`
}

// SilenceSpec defines the desired state of Silence.
// TODO (user): Add fields to SilenceSpec to represent the actual silence api.
type SilenceSpec struct {
	// Matchers defines the alert matchers that this silence will apply to.
	Matchers []SilenceMatcher `json:"matchers"`
	// Owner is GitHub username of a person who created and/or owns the silence.
	// +optional
	Owner string `json:"owner,omitempty"`
	// IssueURL is a link to a GitHub issue describing the problem.
	// +optional
	IssueURL string `json:"issue_url,omitempty"`
}

// Silence is the Schema for the silences API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Owner",type=string,JSONPath=`.spec.owner`
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
