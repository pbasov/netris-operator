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
	"fmt"

	k8sv1alpha1 "github.com/netrisai/netris-operator/api/v1alpha1"
	"github.com/netrisai/netriswebapi/v2/types/servercluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServerClusterToMeta converts the ServerCluster resource to Meta type.
func (r *ServerClusterReconciler) ServerClusterToMeta(cluster *k8sv1alpha1.ServerCluster) (*k8sv1alpha1.ServerClusterMeta, error) {
	var (
		imported = false
		reclaim  = false
	)

	if i, ok := cluster.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = true
	}
	if i, ok := cluster.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = true
	}

	siteID := 0
	siteName := cluster.Spec.Site
	if site, ok := r.NStorage.SitesStorage.FindByName(cluster.Spec.Site); ok {
		siteID = site.ID
		siteName = site.Name
	} else {
		return nil, fmt.Errorf("couldn't find site '%s'", cluster.Spec.Site)
	}

	vpcID := 0
	vpcName := ""
	if cluster.Spec.VPC != "" {
		if vpc, ok := r.NStorage.VPCStorage.FindByName(cluster.Spec.VPC); ok {
			vpcID = vpc.ID
			vpcName = vpc.Name
		} else {
			return nil, fmt.Errorf("couldn't find vpc '%s'", cluster.Spec.VPC)
		}
	}

	templateID := 0
	templateName := ""
	if cluster.Spec.Template != "" {
		if template, ok := r.NStorage.ServerClusterTemplateStorage.FindByName(cluster.Spec.Template); ok {
			templateID = template.ID
			templateName = template.Name
		} else {
			return nil, fmt.Errorf("couldn't find server cluster template '%s'", cluster.Spec.Template)
		}
	}

	adminID := 0
	adminName := ""
	if cluster.Spec.Admin != "" {
		if tenant, ok := r.NStorage.TenantsStorage.FindByName(cluster.Spec.Admin); ok {
			adminID = tenant.ID
			adminName = tenant.Name
		} else {
			return nil, fmt.Errorf("couldn't find admin tenant '%s'", cluster.Spec.Admin)
		}
	}

	// Convert servers to meta format
	servers := []servercluster.Servers{}
	for _, srv := range cluster.Spec.Servers {
		serverID := 0
		if hw, ok := r.NStorage.HWsStorage.FindServerByName(srv.Name); ok {
			serverID = hw.ID
		} else {
			return nil, fmt.Errorf("couldn't find server '%s'", srv.Name)
		}
		servers = append(servers, servercluster.Servers{
			ID:   serverID,
			Name: srv.Name,
		})
	}

	clusterMeta := &k8sv1alpha1.ServerClusterMeta{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(cluster.GetUID()),
			Namespace: cluster.GetNamespace(),
		},
		TypeMeta: metav1.TypeMeta{},
		Spec: k8sv1alpha1.ServerClusterMetaSpec{
			Imported:          imported,
			Reclaim:           reclaim,
			ServerClusterName: cluster.Name,
			SiteID:            siteID,
			SiteName:          siteName,
			AdminID:           adminID,
			AdminName:         adminName,
			VPCID:             vpcID,
			VPCName:           vpcName,
			TemplateID:        templateID,
			TemplateName:      templateName,
			Tags:              cluster.Spec.Tags,
			Servers:           servers,
		},
	}

	return clusterMeta, nil
}

func serverClusterCompareFieldsForNewMeta(cluster *k8sv1alpha1.ServerCluster, clusterMeta *k8sv1alpha1.ServerClusterMeta) bool {
	imported := false
	reclaim := false
	if i, ok := cluster.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = true
	}
	if i, ok := cluster.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = true
	}
	return cluster.GetGeneration() != clusterMeta.Spec.ServerClusterCRGeneration || imported != clusterMeta.Spec.Imported || reclaim != clusterMeta.Spec.Reclaim
}

func serverClusterMustUpdateAnnotations(cluster *k8sv1alpha1.ServerCluster) bool {
	update := false
	if i, ok := cluster.GetAnnotations()["resource.k8s.netris.ai/import"]; !(ok && (i == "true" || i == "false")) {
		update = true
	}
	if i, ok := cluster.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; !(ok && (i == "retain" || i == "delete")) {
		update = true
	}
	return update
}

func serverClusterUpdateDefaultAnnotations(cluster *k8sv1alpha1.ServerCluster) {
	imported := "false"
	reclaim := "delete"
	if i, ok := cluster.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = "true"
	}
	if i, ok := cluster.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = "retain"
	}
	annotations := cluster.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["resource.k8s.netris.ai/import"] = imported
	annotations["resource.k8s.netris.ai/reclaimPolicy"] = reclaim
	cluster.SetAnnotations(annotations)
}

// ServerClusterMetaToNetris converts Meta to Netris API type.
func ServerClusterMetaToNetris(clusterMeta *k8sv1alpha1.ServerClusterMeta) *servercluster.ServerClusterW {
	return &servercluster.ServerClusterW{
		Name: clusterMeta.Spec.ServerClusterName,
		Admin: servercluster.IDName{
			ID:   clusterMeta.Spec.AdminID,
			Name: clusterMeta.Spec.AdminName,
		},
		Site: servercluster.IDName{
			ID:   clusterMeta.Spec.SiteID,
			Name: clusterMeta.Spec.SiteName,
		},
		VPC: servercluster.IDName{
			ID:   clusterMeta.Spec.VPCID,
			Name: clusterMeta.Spec.VPCName,
		},
		SrvClusterTemplate: servercluster.IDName{
			ID:   clusterMeta.Spec.TemplateID,
			Name: clusterMeta.Spec.TemplateName,
		},
		Tags:    clusterMeta.Spec.Tags,
		Servers: clusterMeta.Spec.Servers,
	}
}

// ServerClusterMetaToNetrisUpdate converts Meta to Netris API update type.
func ServerClusterMetaToNetrisUpdate(clusterMeta *k8sv1alpha1.ServerClusterMeta) *servercluster.ServerClusterU {
	return &servercluster.ServerClusterU{
		Tags:    clusterMeta.Spec.Tags,
		Servers: clusterMeta.Spec.Servers,
	}
}

func compareServerClusterMetaAPI(clusterMeta *k8sv1alpha1.ServerClusterMeta, apiCluster *servercluster.ServerCluster, u uniReconciler) bool {
	if apiCluster.Name != clusterMeta.Spec.ServerClusterName {
		u.DebugLogger.Info("Name changed", "netrisValue", apiCluster.Name, "k8sValue", clusterMeta.Spec.ServerClusterName)
		return false
	}
	if apiCluster.Site.ID != clusterMeta.Spec.SiteID {
		u.DebugLogger.Info("SiteID changed", "netrisValue", apiCluster.Site.ID, "k8sValue", clusterMeta.Spec.SiteID)
		return false
	}
	if apiCluster.VPC.ID != clusterMeta.Spec.VPCID {
		u.DebugLogger.Info("VPCID changed", "netrisValue", apiCluster.VPC.ID, "k8sValue", clusterMeta.Spec.VPCID)
		return false
	}
	if apiCluster.SrvClusterTemplate.ID != clusterMeta.Spec.TemplateID {
		u.DebugLogger.Info("TemplateID changed", "netrisValue", apiCluster.SrvClusterTemplate.ID, "k8sValue", clusterMeta.Spec.TemplateID)
		return false
	}
	return true
}
