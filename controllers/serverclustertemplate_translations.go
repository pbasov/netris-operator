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

package controllers

import (
	k8sv1alpha1 "github.com/netrisai/netris-operator/api/v1alpha1"
	"github.com/netrisai/netriswebapi/v2/types/serverclustertemplate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServerClusterTemplateToMeta converts the ServerClusterTemplate resource to Meta type.
func (r *ServerClusterTemplateReconciler) ServerClusterTemplateToMeta(template *k8sv1alpha1.ServerClusterTemplate) (*k8sv1alpha1.ServerClusterTemplateMeta, error) {
	var (
		imported = false
		reclaim  = false
	)

	if i, ok := template.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = true
	}
	if i, ok := template.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = true
	}

	templateMeta := &k8sv1alpha1.ServerClusterTemplateMeta{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(template.GetUID()),
			Namespace: template.GetNamespace(),
		},
		TypeMeta: metav1.TypeMeta{},
		Spec: k8sv1alpha1.ServerClusterTemplateMetaSpec{
			Imported:                  imported,
			Reclaim:                   reclaim,
			ServerClusterTemplateName: template.Name,
			VNets:                     template.Spec.VNets,
		},
	}

	return templateMeta, nil
}

func serverClusterTemplateCompareFieldsForNewMeta(template *k8sv1alpha1.ServerClusterTemplate, templateMeta *k8sv1alpha1.ServerClusterTemplateMeta) bool {
	imported := false
	reclaim := false
	if i, ok := template.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = true
	}
	if i, ok := template.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = true
	}
	return template.GetGeneration() != templateMeta.Spec.ServerClusterTemplateCRGeneration || imported != templateMeta.Spec.Imported || reclaim != templateMeta.Spec.Reclaim
}

func serverClusterTemplateMustUpdateAnnotations(template *k8sv1alpha1.ServerClusterTemplate) bool {
	update := false
	if i, ok := template.GetAnnotations()["resource.k8s.netris.ai/import"]; !(ok && (i == "true" || i == "false")) {
		update = true
	}
	if i, ok := template.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; !(ok && (i == "retain" || i == "delete")) {
		update = true
	}
	return update
}

func serverClusterTemplateUpdateDefaultAnnotations(template *k8sv1alpha1.ServerClusterTemplate) {
	imported := "false"
	reclaim := "delete"
	if i, ok := template.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = "true"
	}
	if i, ok := template.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = "retain"
	}
	annotations := template.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["resource.k8s.netris.ai/import"] = imported
	annotations["resource.k8s.netris.ai/reclaimPolicy"] = reclaim
	template.SetAnnotations(annotations)
}

// ServerClusterTemplateMetaToNetris converts Meta to Netris API type.
func ServerClusterTemplateMetaToNetris(templateMeta *k8sv1alpha1.ServerClusterTemplateMeta) *serverclustertemplate.ServerClusterTemplateW {
	// Convert VNets struct slice to interface slice for API compatibility
	vnets := make([]interface{}, 0, len(templateMeta.Spec.VNets))
	for _, vnet := range templateMeta.Spec.VNets {
		vnetMap := map[string]interface{}{
			"postfix":    vnet.Postfix,
			"type":       vnet.Type,
			"serverNics": vnet.ServerNics,
		}

		if vnet.Vlan != "" {
			vnetMap["vlan"] = vnet.Vlan
		}
		if vnet.VlanID != "" {
			vnetMap["vlanID"] = vnet.VlanID
		}
		if vnet.IPv4DhcpEnabled {
			vnetMap["ipv4DhcpEnabled"] = vnet.IPv4DhcpEnabled
		}
		if vnet.IPv6DhcpEnabled {
			vnetMap["ipv6DhcpEnabled"] = vnet.IPv6DhcpEnabled
		}

		if vnet.IPv4Gateway != nil {
			gw := map[string]interface{}{}
			if vnet.IPv4Gateway.AssignType != "" {
				gw["assignType"] = vnet.IPv4Gateway.AssignType
			}
			if vnet.IPv4Gateway.Allocation != "" {
				gw["allocation"] = vnet.IPv4Gateway.Allocation
			}
			if vnet.IPv4Gateway.ChildSubnetPrefixLength > 0 {
				gw["childSubnetPrefixLength"] = vnet.IPv4Gateway.ChildSubnetPrefixLength
			}
			if vnet.IPv4Gateway.Hostnum > 0 {
				gw["hostnum"] = vnet.IPv4Gateway.Hostnum
			}
			if len(gw) > 0 {
				vnetMap["ipv4Gateway"] = gw
			}
		}

		if vnet.IPv6Gateway != nil {
			gw := map[string]interface{}{}
			if vnet.IPv6Gateway.AssignType != "" {
				gw["assignType"] = vnet.IPv6Gateway.AssignType
			}
			if vnet.IPv6Gateway.Allocation != "" {
				gw["allocation"] = vnet.IPv6Gateway.Allocation
			}
			if vnet.IPv6Gateway.ChildSubnetPrefixLength > 0 {
				gw["childSubnetPrefixLength"] = vnet.IPv6Gateway.ChildSubnetPrefixLength
			}
			if vnet.IPv6Gateway.Hostnum > 0 {
				gw["hostnum"] = vnet.IPv6Gateway.Hostnum
			}
			if len(gw) > 0 {
				vnetMap["ipv6Gateway"] = gw
			}
		}

		vnets = append(vnets, vnetMap)
	}

	return &serverclustertemplate.ServerClusterTemplateW{
		Name:  templateMeta.Spec.ServerClusterTemplateName,
		Vnets: vnets,
	}
}

func compareServerClusterTemplateMetaAPI(templateMeta *k8sv1alpha1.ServerClusterTemplateMeta, apiTemplate *serverclustertemplate.ServerClusterTemplate, u uniReconciler) bool {
	if apiTemplate.Name != templateMeta.Spec.ServerClusterTemplateName {
		u.DebugLogger.Info("Name changed", "netrisValue", apiTemplate.Name, "k8sValue", templateMeta.Spec.ServerClusterTemplateName)
		return false
	}
	// VNets comparison is complex due to interface{} type,
	// we rely on generation comparison for updates
	return true
}
