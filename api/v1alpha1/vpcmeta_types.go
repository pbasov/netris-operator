/*
Copyright 2021. Netris, Inc.

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

// VPCGuestTenant represents a tenant with access to the VPC
type VPCGuestTenant struct {
	// ID is the resolved tenant ID
	ID int `json:"id"`
	// Name is the tenant name
	Name string `json:"name"`
}

// VPCMetaSpec defines the desired state of VPCMeta
type VPCMetaSpec struct {
	// Imported indicates if this resource was imported from existing Netris
	Imported bool `json:"imported"`
	// Reclaim indicates if the resource should be retained when the CR is deleted
	Reclaim bool `json:"reclaimPolicy"`
	// VPCCRGeneration tracks the generation of the parent CR
	VPCCRGeneration int64 `json:"vpcGeneration"`
	// ID is the Netris API ID
	ID int `json:"id"`
	// VPCName is the name of the parent CR
	VPCName string `json:"vpcName"`
	// AdminTenantID is the resolved admin tenant ID
	AdminTenantID int `json:"adminTenantId"`
	// AdminTenantName is the admin tenant name
	AdminTenantName string `json:"adminTenantName"`
	// GuestTenants is the list of guest tenants with resolved IDs
	GuestTenants []VPCGuestTenant `json:"guestTenants,omitempty"`
	// Tags are labels for the VPC
	Tags []string `json:"tags,omitempty"`
}

// VPCMetaStatus defines the observed state of VPCMeta
type VPCMetaStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VPCMeta is the Schema for the vpcsmeta API
type VPCMeta struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCMetaSpec   `json:"spec,omitempty"`
	Status VPCMetaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VPCMetaList contains a list of VPCMeta
type VPCMetaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCMeta `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCMeta{}, &VPCMetaList{})
}
