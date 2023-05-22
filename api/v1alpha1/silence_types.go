package v1alpha1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=common;giantswarm
// +kubebuilder:storageversion
// +k8s:openapi-gen=true
// Silence represents schema for managed silences in Alertmanager. Reconciled by silence-operator.
type Silence struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              SilenceSpec `json:"spec"`
}

// +k8s:openapi-gen=true
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
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual,omitempty"`
	Name    string `json:"name"`
	Value   string `json:"value"`
}

func (m *Matcher) UnmarshalJSON(text []byte) error {
	type innerMatcher Matcher

	// We check for equality by default to keep the API
	matcher := &innerMatcher{
		IsEqual: true,
	}
	if err := json.Unmarshal(text, matcher); err != nil {
		return err
	}
	*m = Matcher(*matcher)
	return nil
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SilenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Silence `json:"items"`
}
