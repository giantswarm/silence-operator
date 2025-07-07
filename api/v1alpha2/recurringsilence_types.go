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
	// Schedule is the cron expression for the recurring silence.
	Schedule string `json:"schedule"`

	// Template is the template for the silence to be created.
	Template SilenceSpec `json:"template"`
}

// RecurringSilence is the Schema for the recurringsilences API.
// +kubebuilder:object:root=true
type RecurringSilence struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec RecurringSilenceSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// RecurringSilenceList contains a list of RecurringSilence.
type RecurringSilenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RecurringSilence `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RecurringSilence{}, &RecurringSilenceList{})
}
