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
	"github.com/netrisai/netriswebapi/v2/types/inventory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InventoryServerMetaSpec defines the desired state of InventoryServerMeta
type InventoryServerMetaSpec struct {
	// Imported indicates if this resource was imported from existing Netris inventory
	Imported bool `json:"imported"`
	// Reclaim indicates if the resource should be retained when the CR is deleted
	Reclaim bool `json:"reclaimPolicy"`
	// InventoryServerCRGeneration tracks the generation of the parent CR
	InventoryServerCRGeneration int64 `json:"inventoryServerGeneration"`
	// ID is the Netris API ID of the inventory server
	ID int `json:"id"`
	// InventoryServerName is the name of the parent InventoryServer CR
	InventoryServerName string `json:"inventoryServerName"`

	// TenantID is the resolved tenant ID
	TenantID int `json:"tenant,omitempty"`
	// Description of the server
	Description string `json:"description,omitempty"`
	// SiteID is the resolved site ID
	SiteID int `json:"site,omitempty"`
	// ProfileID is the resolved inventory profile ID
	ProfileID int `json:"profile,omitempty"`
	// MainIP is the main IP address
	MainIP string `json:"mainIp,omitempty"`
	// MgmtIP is the management IP address
	MgmtIP string `json:"mgmtIp,omitempty"`
	// ASN is the autonomous system number
	ASN int `json:"asn,omitempty"`
	// PortsCount is the number of physical ports
	PortsCount int `json:"portsCount,omitempty"`
	// UUID is the unique identifier
	UUID string `json:"uuid,omitempty"`
	// Links are the physical connections with resolved IDs
	Links []inventory.HWLink `json:"links,omitempty"`
	// CustomData is arbitrary custom data
	CustomData string `json:"customData,omitempty"`
	// Tags are labels for the server
	Tags []string `json:"tags,omitempty"`
	// SRVRole is the server role
	SRVRole string `json:"srvRole,omitempty"`
}

// InventoryServerMetaStatus defines the observed state of InventoryServerMeta
type InventoryServerMetaStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// InventoryServerMeta is the Schema for the inventoryservermeta API
type InventoryServerMeta struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InventoryServerMetaSpec   `json:"spec,omitempty"`
	Status InventoryServerMetaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InventoryServerMetaList contains a list of InventoryServerMeta
type InventoryServerMetaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InventoryServerMeta `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InventoryServerMeta{}, &InventoryServerMetaList{})
}
