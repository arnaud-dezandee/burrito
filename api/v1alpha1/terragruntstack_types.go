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

type TerragruntStackSpec struct {
	Path                 string                   `json:"path,omitempty"`
	Branch               string                   `json:"branch,omitempty"`
	AdditionalTargetRefs []string                 `json:"additionalTargetRefs,omitempty"`
	Parallelism          *int                     `json:"parallelism,omitempty"`
	TerraformConfig      TerraformConfig          `json:"terraform,omitempty"`
	OpenTofuConfig       OpenTofuConfig           `json:"opentofu,omitempty"`
	TerragruntConfig     TerragruntConfig         `json:"terragrunt,omitempty"`
	Repository           TerraformLayerRepository `json:"repository,omitempty"`
	RemediationStrategy  RemediationStrategy      `json:"remediationStrategy,omitempty"`
	OverrideRunnerSpec   OverrideRunnerSpec       `json:"overrideRunnerSpec,omitempty"`
	RunHistoryPolicy     RunHistoryPolicy         `json:"runHistoryPolicy,omitempty"`
}

type TerragruntStackStatus struct {
	Conditions []metav1.Condition      `json:"conditions,omitempty"`
	State      string                  `json:"state,omitempty"`
	LastResult string                  `json:"lastResult,omitempty"`
	LastRun    TerragruntStackRunRef   `json:"lastRun,omitempty"`
	LatestRuns []TerragruntStackRunRef `json:"latestRuns,omitempty"`
	Units      []TerragruntStackUnit   `json:"units,omitempty"`
}

type TerragruntStackRunRef struct {
	Name   string      `json:"name,omitempty"`
	Commit string      `json:"commit,omitempty"`
	Date   metav1.Time `json:"date,omitempty"`
	Action string      `json:"action,omitempty"`
}

type TerragruntStackUnit struct {
	ID                  string                 `json:"id,omitempty"`
	Path                string                 `json:"path,omitempty"`
	State               string                 `json:"state,omitempty"`
	LastAction          string                 `json:"lastAction,omitempty"`
	LastRun             string                 `json:"lastRun,omitempty"`
	LastRunAt           metav1.Time            `json:"lastRunAt,omitempty"`
	LastResult          string                 `json:"lastResult,omitempty"`
	HasValidPlan        bool                   `json:"hasValidPlan,omitempty"`
	LastPlannedRevision string                 `json:"lastPlannedRevision,omitempty"`
	LastAppliedRevision string                 `json:"lastAppliedRevision,omitempty"`
	IsRunning           bool                   `json:"isRunning,omitempty"`
	LatestRuns          []TerragruntUnitRunRef `json:"latestRuns,omitempty"`
}

type TerragruntUnitRunRef struct {
	Run    string      `json:"run,omitempty"`
	Action string      `json:"action,omitempty"`
	Date   metav1.Time `json:"date,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=tgstacks;tgstack;
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Repository",type=string,JSONPath=`.spec.repository.name`
// +kubebuilder:printcolumn:name="Branch",type=string,JSONPath=`.spec.branch`
// +kubebuilder:printcolumn:name="Path",type=string,JSONPath=`.spec.path`
type TerragruntStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TerragruntStackSpec   `json:"spec,omitempty"`
	Status TerragruntStackStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type TerragruntStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TerragruntStack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TerragruntStack{}, &TerragruntStackList{})
}

func (stack *TerragruntStack) GetAPIVersion() string {
	if stack.APIVersion == "" {
		return "config.terraform.padok.cloud/v1alpha1"
	}
	return stack.APIVersion
}

func (stack *TerragruntStack) GetKind() string {
	if stack.Kind == "" {
		return "TerragruntStack"
	}
	return stack.Kind
}
