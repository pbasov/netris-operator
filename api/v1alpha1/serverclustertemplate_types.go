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

// ServerClusterTemplateVNetGateway defines IPv4/IPv6 gateway configuration for a VNet
type ServerClusterTemplateVNetGateway struct {
	// AssignType specifies how the gateway is assigned ("auto" or "manual")
	// +kubebuilder:validation:Enum=auto;manual
	AssignType string `json:"assignType,omitempty"`
	// Allocation is the IP allocation pool (e.g., "10.188.0.0/16")
	Allocation string `json:"allocation,omitempty"`
	// ChildSubnetPrefixLength is the prefix length for child subnets
	ChildSubnetPrefixLength int `json:"childSubnetPrefixLength,omitempty"`
	// Hostnum is the host number for the gateway
	Hostnum int `json:"hostnum,omitempty"`
}

// ServerClusterTemplateVNet defines a VNet configuration within the template
type ServerClusterTemplateVNet struct {
	// Postfix is appended to the cluster name to form the VNet name
	Postfix string `json:"postfix"`
	// Type is the VNet type (e.g., "l2vpn", "l3vpn")
	// +kubebuilder:validation:Enum=l2vpn;l3vpn
	Type string `json:"type"`
	// ServerNics lists the server NICs to connect to this VNet
	ServerNics []string `json:"serverNics"`
	// Vlan specifies VLAN mode ("tagged" or "untagged")
	// +kubebuilder:validation:Enum=tagged;untagged
	Vlan string `json:"vlan,omitempty"`
	// VlanID is the VLAN ID ("auto" or a specific number)
	VlanID string `json:"vlanID,omitempty"`
	// IPv4Gateway configures the IPv4 gateway for this VNet
	IPv4Gateway *ServerClusterTemplateVNetGateway `json:"ipv4Gateway,omitempty"`
	// IPv6Gateway configures the IPv6 gateway for this VNet
	IPv6Gateway *ServerClusterTemplateVNetGateway `json:"ipv6Gateway,omitempty"`
	// IPv4DhcpEnabled enables DHCP for IPv4
	IPv4DhcpEnabled bool `json:"ipv4DhcpEnabled,omitempty"`
	// IPv6DhcpEnabled enables DHCP for IPv6
	IPv6DhcpEnabled bool `json:"ipv6DhcpEnabled,omitempty"`
}

// ServerClusterTemplateSpec defines the desired state of ServerClusterTemplate
type ServerClusterTemplateSpec struct {
	// VNets defines the list of VNet configurations for this template
	VNets []ServerClusterTemplateVNet `json:"vnets,omitempty"`
}

// ServerClusterTemplateStatus defines the observed state of ServerClusterTemplate
type ServerClusterTemplateStatus struct {
	// Status represents the current provisioning status
	Status string `json:"status,omitempty"`
	// Message provides additional information about the status
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="VNets",type=integer,JSONPath=`.spec.vnets`,description="Number of VNets"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ServerClusterTemplate is the Schema for the serverclustertemplates API
type ServerClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServerClusterTemplateSpec   `json:"spec,omitempty"`
	Status ServerClusterTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServerClusterTemplateList contains a list of ServerClusterTemplate
type ServerClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServerClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServerClusterTemplate{}, &ServerClusterTemplateList{})
}
