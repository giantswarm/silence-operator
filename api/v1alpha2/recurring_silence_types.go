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

// RecurringSilenceSpec defines the desired state of RecurringSilence.
type RecurringSilenceSpec struct {
	// Schedule defines when the silence should be created using cron expression format.
	// Supports 5-field format: "* * * * *" (minute hour day-of-month month day-of-week).
	// Examples:
	//   - "0 0 * * *" - Daily at midnight
	//   - "0 2 * * 1" - Weekly on Monday at 2 AM
	//   - "30 14 1 * *" - Monthly on the 1st at 2:30 PM
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^(\S+\s+\S+\s+\S+\s+\S+\s+\S+)$`
	Schedule string `json:"schedule"`

	// Duration specifies how long each silence should last.
	// Must be a valid Go duration string (e.g., "30m", "2h", "24h").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$`
	Duration string `json:"duration"`

	// Matchers defines the alert matchers that the generated silences will apply to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Matchers []SilenceMatcher `json:"matchers"`
}

// RecurringSilenceStatus defines the observed state of RecurringSilence.
type RecurringSilenceStatus struct {
	// LastScheduledTime indicates the last time a silence was scheduled from this RecurringSilence.
	// +optional
	LastScheduledTime *metav1.Time `json:"lastScheduledTime,omitempty"`

	// NextScheduledTime indicates the next time a silence will be scheduled from this RecurringSilence.
	// +optional
	NextScheduledTime *metav1.Time `json:"nextScheduledTime,omitempty"`

	// ActiveSilence indicates the name of the currently active silence created by this RecurringSilence.
	// +optional
	ActiveSilence *string `json:"activeSilence,omitempty"`

	// Conditions represent the latest available observations of the RecurringSilence's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// RecurringSilence is the Schema for the recurring silences API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Duration",type=string,JSONPath=`.spec.duration`
// +kubebuilder:printcolumn:name="Active Silence",type=string,JSONPath=`.status.activeSilence`
// +kubebuilder:printcolumn:name="Next Scheduled",type=date,JSONPath=`.status.nextScheduledTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type RecurringSilence struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RecurringSilenceSpec   `json:"spec,omitempty"`
	Status RecurringSilenceStatus `json:"status,omitempty"`
}

// RecurringSilenceList contains a list of RecurringSilence.
// +kubebuilder:object:root=true
type RecurringSilenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RecurringSilence `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RecurringSilence{}, &RecurringSilenceList{})
}