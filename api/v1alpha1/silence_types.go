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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SilenceSpec defines the desired state of Silence.
type SilenceSpec struct {
	TargetTags []TargetTag `json:"targetTags,omitempty"`
	Matchers   []Matcher   `json:"matchers"`

	// Owner is GitHub username of a person who created and/or owns the silence.
	Owner string `json:"owner,omitempty"`

	// PostmortemURL is a link to a document describing the problem.
	// Deprecated: Use IssueURL instead.
	PostmortemURL *string `json:"postmortem_url,omitempty"`

	// IssueURL is a link to a GitHub issue describing the problem.
	IssueURL string `json:"issue_url,omitempty"`
}

type TargetTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Matcher struct {
	IsRegex bool   `json:"isRegex,omitempty"`
	IsEqual *bool  `json:"isEqual,omitempty"`
	Name    string `json:"name"`
	Value   string `json:"value"`
}

// Silence is the Schema for the silences API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type Silence struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec SilenceSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// SilenceList contains a list of Silence.
type SilenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Silence `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Silence{}, &SilenceList{})
}
