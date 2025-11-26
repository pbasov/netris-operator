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

// ServerClusterTemplateMetaSpec defines the desired state of ServerClusterTemplateMeta
type ServerClusterTemplateMetaSpec struct {
	// Imported indicates if this resource was imported from existing Netris
	Imported bool `json:"imported"`
	// Reclaim indicates if the resource should be retained when the CR is deleted
	Reclaim bool `json:"reclaimPolicy"`
	// ServerClusterTemplateCRGeneration tracks the generation of the parent CR
	ServerClusterTemplateCRGeneration int64 `json:"serverClusterTemplateGeneration"`
	// ID is the Netris API ID
	ID int `json:"id"`
	// ServerClusterTemplateName is the name of the parent CR
	ServerClusterTemplateName string `json:"serverClusterTemplateName"`
	// VNets is the VNet configuration
	VNets []ServerClusterTemplateVNet `json:"vnets,omitempty"`
}

// ServerClusterTemplateMetaStatus defines the observed state of ServerClusterTemplateMeta
type ServerClusterTemplateMetaStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ServerClusterTemplateMeta is the Schema for the serverclustertemplatesmeta API
type ServerClusterTemplateMeta struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServerClusterTemplateMetaSpec   `json:"spec,omitempty"`
	Status ServerClusterTemplateMetaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServerClusterTemplateMetaList contains a list of ServerClusterTemplateMeta
type ServerClusterTemplateMetaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServerClusterTemplateMeta `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServerClusterTemplateMeta{}, &ServerClusterTemplateMetaList{})
}
