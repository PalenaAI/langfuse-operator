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

// LangfuseOrganizationSpec defines the desired state of LangfuseOrganization.
type LangfuseOrganizationSpec struct {
	// InstanceRef references the parent LangfuseInstance.
	InstanceRef ObjectReference `json:"instanceRef"`
	// DisplayName is the organization name in Langfuse.
	DisplayName string `json:"displayName"`
	// Members configures organization membership.
	// +optional
	Members OrganizationMembersSpec `json:"members,omitempty"`
}

// OrganizationMembersSpec defines how organization members are managed.
type OrganizationMembersSpec struct {
	// ManagedExclusively when true removes users not in this list.
	// +optional
	ManagedExclusively bool `json:"managedExclusively,omitempty"`
	// Users is the list of organization members.
	// +optional
	Users []OrganizationMemberSpec `json:"users,omitempty"`
}

// OrganizationMemberSpec defines a single organization member.
type OrganizationMemberSpec struct {
	// Email is the user's email address.
	Email string `json:"email"`
	// Role is the user's role in the organization.
	// +kubebuilder:validation:Enum=owner;admin;member;viewer
	Role string `json:"role"`
}

// LangfuseOrganizationStatus defines the observed state of LangfuseOrganization.
type LangfuseOrganizationStatus struct {
	// Ready indicates whether the organization is fully synced.
	Ready bool `json:"ready,omitempty"`
	// OrganizationID is the Langfuse internal organization ID.
	// +optional
	OrganizationID string `json:"organizationId,omitempty"`
	// MemberCount is the total number of members.
	MemberCount int `json:"memberCount,omitempty"`
	// SyncedMembers is the number of confirmed synced members.
	SyncedMembers int `json:"syncedMembers,omitempty"`
	// ProjectCount is the number of LangfuseProject CRs referencing this org.
	ProjectCount int `json:"projectCount,omitempty"`
	// Conditions represent the latest observations of the organization's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Org ID",type=string,JSONPath=`.status.organizationId`
// +kubebuilder:printcolumn:name="Members",type=integer,JSONPath=`.status.memberCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LangfuseOrganization is the Schema for the langfuseorganizations API.
type LangfuseOrganization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LangfuseOrganizationSpec   `json:"spec,omitempty"`
	Status LangfuseOrganizationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LangfuseOrganizationList contains a list of LangfuseOrganization.
type LangfuseOrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LangfuseOrganization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LangfuseOrganization{}, &LangfuseOrganizationList{})
}
