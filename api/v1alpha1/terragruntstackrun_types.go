/*
Copyright 2022.

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

type TerragruntStackRunSpec struct {
	Action string                  `json:"action,omitempty"`
	Stack  TerragruntStackRunStack `json:"stack,omitempty"`
}

type TerragruntStackRunStack struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Revision  string `json:"revision,omitempty"`
}

type TerragruntStackRunStatus struct {
	Conditions  []metav1.Condition          `json:"conditions,omitempty"`
	State       string                      `json:"state,omitempty"`
	Retries     int                         `json:"retries"`
	LastRun     string                      `json:"lastRun,omitempty"`
	Attempts    []Attempt                   `json:"attempts,omitempty"`
	RunnerPod   string                      `json:"runnerPod,omitempty"`
	UnitResults []TerragruntStackUnitResult `json:"unitResults,omitempty"`
}

type TerragruntStackUnitResult struct {
	Run                 string      `json:"run,omitempty"`
	ID                  string      `json:"id,omitempty"`
	Path                string      `json:"path,omitempty"`
	State               string      `json:"state,omitempty"`
	Action              string      `json:"action,omitempty"`
	Result              string      `json:"result,omitempty"`
	HasValidPlan        bool        `json:"hasValidPlan,omitempty"`
	LastPlannedRevision string      `json:"lastPlannedRevision,omitempty"`
	LastAppliedRevision string      `json:"lastAppliedRevision,omitempty"`
	RunAt               metav1.Time `json:"runAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=tgstackruns;tgstackrun;
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Retries",type=integer,JSONPath=`.status.retries`
// +kubebuilder:printcolumn:name="Created On",type=string,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Runner Pod",type=string,JSONPath=`.status.runnerPod`
type TerragruntStackRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TerragruntStackRunSpec   `json:"spec,omitempty"`
	Status TerragruntStackRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type TerragruntStackRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TerragruntStackRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TerragruntStackRun{}, &TerragruntStackRunList{})
}

func (run *TerragruntStackRun) GetAPIVersion() string {
	if run.APIVersion == "" {
		return "config.terraform.padok.cloud/v1alpha1"
	}
	return run.APIVersion
}

func (run *TerragruntStackRun) GetKind() string {
	if run.Kind == "" {
		return "TerragruntStackRun"
	}
	return run.Kind
}
