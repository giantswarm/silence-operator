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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RecurringSilenceSpec defines the desired state of RecurringSilence.
type RecurringSilenceSpec struct {
	// Schedule is the cron expression for the recurring silence.
	// It follows the format of https://github.com/aptible/supercronic.
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`

	// TimeZone for the cron schedule.
	// If not specified, the controller will use its local time zone.
	// +optional
	TimeZone string `json:"timeZone,omitempty"`

	// SilenceTemplate defines the template for the Silence object to be created.
	// +kubebuilder:validation:Required
	SilenceTemplate SilenceSpec `json:"silenceTemplate"`
}

// RecurringSilenceStatus defines the observed state of RecurringSilence.
type RecurringSilenceStatus struct {
	// LastScheduleTime is the last time a Silence was successfully created.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// Active is a list of references to Silence objects created by this RecurringSilence.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RecurringSilence is the Schema for the recurringsilences API
type RecurringSilence struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RecurringSilenceSpec   `json:"spec,omitempty"`
	Status RecurringSilenceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RecurringSilenceList contains a list of RecurringSilence
type RecurringSilenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RecurringSilence `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RecurringSilence{}, &RecurringSilenceList{})
}
