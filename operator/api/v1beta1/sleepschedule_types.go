/*
Copyright 2024 Peter Valdez.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type SleepScheduleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The time that the deployment should wake up
	// +kubebuilder:validation:Required
	WakeTime string `json:"wakeTime"`

	// The time that the deployment should sleep
	// +kubebuilder:validation:Required
	SleepTime string `json:"sleepTime"`

	// The timezone that the times above should be based in
	// +kubebuilder:validation:Required
	Timezone string `json:"timezone"`

	// The name of the deployment that should be slept/woken
	// +kubebuilder:validation:Required
	DeploymentName string `json:"deploymentName,omitempty"`

	// The number of replicas that the deployment with it wakes
	// +kubebuilder:validation:Required
	WakeReplicas int `json:"wakeReplicas,omitempty"`

	// The name of the ingress that will be updated to point to the snorlax proxy
	// which wakes the deployment when a requrest is received. A copy of the original
	// is stored in a configmap.
	IngressName string `json:"ingressName,omitempty"`
}

// SleepScheduleStatus defines the observed state of SleepSchedule
type SleepScheduleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Awake bool `json:"awake,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SleepSchedule is the Schema for the sleepschedules API
type SleepSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SleepScheduleSpec   `json:"spec,omitempty"`
	Status SleepScheduleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SleepScheduleList contains a list of SleepSchedule
type SleepScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SleepSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SleepSchedule{}, &SleepScheduleList{})
}
