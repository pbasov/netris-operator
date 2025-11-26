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
	"github.com/netrisai/netriswebapi/v2/types/servercluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServerClusterMetaSpec defines the desired state of ServerClusterMeta
type ServerClusterMetaSpec struct {
	// Imported indicates if this resource was imported from existing Netris
	Imported bool `json:"imported"`
	// Reclaim indicates if the resource should be retained when the CR is deleted
	Reclaim bool `json:"reclaimPolicy"`
	// ServerClusterCRGeneration tracks the generation of the parent CR
	ServerClusterCRGeneration int64 `json:"serverClusterGeneration"`
	// ID is the Netris API ID
	ID int `json:"id"`
	// ServerClusterName is the name of the parent CR
	ServerClusterName string `json:"serverClusterName"`
	// SiteID is the resolved site ID
	SiteID int `json:"siteId"`
	// SiteName is the site name
	SiteName string `json:"siteName"`
	// AdminID is the resolved admin tenant ID
	AdminID int `json:"adminId,omitempty"`
	// AdminName is the admin tenant name
	AdminName string `json:"adminName,omitempty"`
	// VPCID is the resolved VPC ID
	VPCID int `json:"vpcId,omitempty"`
	// VPCName is the VPC name
	VPCName string `json:"vpcName,omitempty"`
	// TemplateID is the resolved template ID
	TemplateID int `json:"templateId"`
	// TemplateName is the template name
	TemplateName string `json:"templateName,omitempty"`
	// Tags are labels for the cluster
	Tags []string `json:"tags,omitempty"`
	// Servers is the list of servers with resolved IDs
	Servers []servercluster.Servers `json:"servers,omitempty"`
}

// ServerClusterMetaStatus defines the observed state of ServerClusterMeta
type ServerClusterMetaStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ServerClusterMeta is the Schema for the serverclustersmeta API
type ServerClusterMeta struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServerClusterMetaSpec   `json:"spec,omitempty"`
	Status ServerClusterMetaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServerClusterMetaList contains a list of ServerClusterMeta
type ServerClusterMetaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServerClusterMeta `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServerClusterMeta{}, &ServerClusterMetaList{})
}
