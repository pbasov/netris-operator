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

// InventoryServerLink defines a link between server NIC and switch port.
type InventoryServerLink struct {
	// Name of the server NIC (e.g., "eth0", "eth1")
	Local string `json:"local"`
	// Switch port in format "portName@switchName" (e.g., "swp1@leaf01")
	Remote string `json:"remote"`
}

// InventoryServerSpec defines the desired state of InventoryServer
type InventoryServerSpec struct {
	// Tenant name that owns this server
	Tenant string `json:"tenant,omitempty"`

	// Description of the server
	Description string `json:"description,omitempty"`

	// Site name where this server is located
	Site string `json:"site"`

	// Profile name for inventory profile configuration
	Profile string `json:"profile,omitempty"`

	// MainIP is the main IP address of the server. Can be "auto" or a specific IP.
	// +kubebuilder:validation:Pattern=`^(auto|((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])))$`
	MainIP string `json:"mainIp,omitempty"`

	// MgmtIP is the management IP address of the server. Can be "auto" or a specific IP.
	// +kubebuilder:validation:Pattern=`^(auto|((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])))$`
	MgmtIP string `json:"mgmtIp,omitempty"`

	// ASN is the autonomous system number for BGP configuration. Use 0 for auto.
	ASN int `json:"asn,omitempty"`

	// PortsCount is the number of physical ports on the server
	// +kubebuilder:validation:Minimum=0
	PortsCount int `json:"portsCount,omitempty"`

	// UUID is the unique identifier of the server (e.g., from IPMI/BMC)
	UUID string `json:"uuid,omitempty"`

	// Links defines the physical connections between server NICs and switch ports
	Links []InventoryServerLink `json:"links,omitempty"`

	// CustomData allows storing arbitrary custom data for the server
	CustomData string `json:"customData,omitempty"`

	// Tags are labels for organizing and filtering servers
	Tags []string `json:"tags,omitempty"`

	// SRVRole is the server role (e.g., "hypervisor", "compute")
	SRVRole string `json:"srvRole,omitempty"`
}

// InventoryServerStatus defines the observed state of InventoryServer
type InventoryServerStatus struct {
	// Status represents the current provisioning status
	Status string `json:"status,omitempty"`
	// Message provides additional information about the status
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Tenant",type=string,JSONPath=`.spec.tenant`
// +kubebuilder:printcolumn:name="Site",type=string,JSONPath=`.spec.site`
// +kubebuilder:printcolumn:name="Profile",type=string,JSONPath=`.spec.profile`
// +kubebuilder:printcolumn:name="Main IP",type=string,JSONPath=`.spec.mainIp`
// +kubebuilder:printcolumn:name="Mgmt IP",type=string,JSONPath=`.spec.mgmtIp`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// InventoryServer is the Schema for the inventoryservers API
type InventoryServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InventoryServerSpec   `json:"spec,omitempty"`
	Status InventoryServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InventoryServerList contains a list of InventoryServer
type InventoryServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InventoryServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InventoryServer{}, &InventoryServerList{})
}
