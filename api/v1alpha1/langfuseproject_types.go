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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LangfuseProjectSpec defines the desired state of LangfuseProject.
type LangfuseProjectSpec struct {
	// InstanceRef references the parent LangfuseInstance.
	InstanceRef ObjectReference `json:"instanceRef"`
	// OrganizationRef references the parent LangfuseOrganization CR.
	OrganizationRef ObjectReference `json:"organizationRef"`
	// ProjectName is the project name in Langfuse.
	ProjectName string `json:"projectName"`
	// APIKeys defines API keys to create and manage.
	// +optional
	APIKeys []APIKeySpec `json:"apiKeys,omitempty"`
}

// APIKeySpec defines an API key to create in Langfuse and store in a K8s Secret.
type APIKeySpec struct {
	// Name is the logical name of the API key.
	Name string `json:"name"`
	// SecretName is the K8s Secret name to store the key pair.
	SecretName string `json:"secretName"`
}

// LangfuseProjectStatus defines the observed state of LangfuseProject.
type LangfuseProjectStatus struct {
	// Ready indicates whether the project is fully synced.
	Ready bool `json:"ready,omitempty"`
	// ProjectID is the Langfuse internal project ID.
	// +optional
	ProjectID string `json:"projectId,omitempty"`
	// OrganizationID is the Langfuse internal organization ID.
	// +optional
	OrganizationID string `json:"organizationId,omitempty"`
	// APIKeys reports the status of each managed API key.
	// +optional
	APIKeys []APIKeyStatus `json:"apiKeys,omitempty"`
	// Conditions represent the latest observations of the project's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// APIKeyStatus reports the status of a single managed API key.
type APIKeyStatus struct {
	// Name is the logical name of the API key.
	Name string `json:"name"`
	// SecretName is the K8s Secret storing the key pair.
	SecretName string `json:"secretName"`
	// Created indicates if the API key has been created.
	Created bool `json:"created"`
	// LastRotated is the last time the API key was rotated.
	// +optional
	LastRotated *metav1.Time `json:"lastRotated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Project ID",type=string,JSONPath=`.status.projectId`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LangfuseProject is the Schema for the langfuseprojects API.
type LangfuseProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LangfuseProjectSpec   `json:"spec,omitempty"`
	Status LangfuseProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LangfuseProjectList contains a list of LangfuseProject.
type LangfuseProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LangfuseProject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LangfuseProject{}, &LangfuseProjectList{})
}
