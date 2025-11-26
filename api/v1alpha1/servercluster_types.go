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

// ServerClusterServer defines a server reference within a cluster
type ServerClusterServer struct {
	// Name is the name of the InventoryServer
	Name string `json:"name"`
	// Shared indicates if the server is shared with other clusters
	Shared bool `json:"shared,omitempty"`
}

// ServerClusterSpec defines the desired state of ServerCluster
type ServerClusterSpec struct {
	// Site is the site name where this cluster is located
	Site string `json:"site"`
	// Admin is the tenant name that administers this cluster
	Admin string `json:"admin,omitempty"`
	// VPC is the VPC name for this cluster
	VPC string `json:"vpc,omitempty"`
	// Template is the name of the ServerClusterTemplate to use
	Template string `json:"template"`
	// Tags are labels for organizing and filtering clusters
	Tags []string `json:"tags,omitempty"`
	// Servers is the list of servers in this cluster
	Servers []ServerClusterServer `json:"servers,omitempty"`
}

// ServerClusterResourceVNet represents a VNet resource created for this cluster
type ServerClusterResourceVNet struct {
	// ID is the Netris ID of the VNet
	ID int `json:"id,omitempty"`
	// Name is the name of the VNet
	Name string `json:"name,omitempty"`
	// IPv4Gateways lists the IPv4 gateway prefixes
	IPv4Gateways []string `json:"ipv4Gateways,omitempty"`
	// IPv6Gateways lists the IPv6 gateway prefixes
	IPv6Gateways []string `json:"ipv6Gateways,omitempty"`
}

// ServerClusterResourceAllocation represents an IP allocation for this cluster
type ServerClusterResourceAllocation struct {
	// ID is the Netris ID of the allocation
	ID int `json:"id,omitempty"`
	// Prefix is the allocated IP prefix
	Prefix string `json:"prefix,omitempty"`
}

// ServerClusterResourceSubnet represents a subnet for this cluster
type ServerClusterResourceSubnet struct {
	// ID is the Netris ID of the subnet
	ID int `json:"id,omitempty"`
	// Prefix is the subnet prefix
	Prefix string `json:"prefix,omitempty"`
}

// ServerClusterResources represents the resources created for this cluster
type ServerClusterResources struct {
	// VNets lists the VNets created for this cluster
	VNets []ServerClusterResourceVNet `json:"vnets,omitempty"`
	// Allocations lists the IP allocations for this cluster
	Allocations []ServerClusterResourceAllocation `json:"allocations,omitempty"`
	// Subnets lists the subnets for this cluster
	Subnets []ServerClusterResourceSubnet `json:"subnets,omitempty"`
}

// ServerClusterStatus defines the observed state of ServerCluster
type ServerClusterStatus struct {
	// Status represents the current provisioning status
	Status string `json:"status,omitempty"`
	// Message provides additional information about the status
	Message string `json:"message,omitempty"`
	// State is the cluster state from Netris
	State string `json:"state,omitempty"`
	// Resources contains the resources created for this cluster
	Resources *ServerClusterResources `json:"resources,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Site",type=string,JSONPath=`.spec.site`
// +kubebuilder:printcolumn:name="VPC",type=string,JSONPath=`.spec.vpc`
// +kubebuilder:printcolumn:name="Template",type=string,JSONPath=`.spec.template`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ServerCluster is the Schema for the serverclusters API
type ServerCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServerClusterSpec   `json:"spec,omitempty"`
	Status ServerClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServerClusterList contains a list of ServerCluster
type ServerClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServerCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServerCluster{}, &ServerClusterList{})
}
