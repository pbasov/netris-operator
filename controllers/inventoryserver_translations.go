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
	"github.com/netrisai/netriswebapi/v2/types/inventory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InventoryServerToInventoryServerMeta converts the InventoryServer resource to InventoryServerMeta type.
func (r *InventoryServerReconciler) InventoryServerToInventoryServerMeta(inventoryServer *k8sv1alpha1.InventoryServer) (*k8sv1alpha1.InventoryServerMeta, error) {
	var (
		imported = false
		reclaim  = false
	)

	if i, ok := inventoryServer.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = true
	}
	if i, ok := inventoryServer.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = true
	}

	siteID := 0
	if site, ok := r.NStorage.SitesStorage.FindByName(inventoryServer.Spec.Site); ok {
		siteID = site.ID
	} else {
		return nil, fmt.Errorf("invalid site '%s'", inventoryServer.Spec.Site)
	}

	tenantID := 0
	if inventoryServer.Spec.Tenant != "" {
		if tenant, ok := r.NStorage.TenantsStorage.FindByName(inventoryServer.Spec.Tenant); ok {
			tenantID = tenant.ID
		} else {
			return nil, fmt.Errorf("invalid tenant '%s'", inventoryServer.Spec.Tenant)
		}
	}

	profileID := 0
	if inventoryServer.Spec.Profile != "" {
		profiles, err := r.Cred.InventoryProfile().Get()
		if err != nil {
			return nil, err
		}

		for _, p := range profiles {
			if p.Name == inventoryServer.Spec.Profile {
				profileID = p.ID
			}
		}

		if profileID == 0 {
			return nil, fmt.Errorf("invalid profile '%s'", inventoryServer.Spec.Profile)
		}
	}

	// Convert links from CRD format to API format
	links := make([]inventory.HWLink, 0, len(inventoryServer.Spec.Links))
	for _, link := range inventoryServer.Spec.Links {
		// Remote is expected in format "portName@switchName"
		// FindByName already searches by "portName@switchName"
		port, ok := r.NStorage.PortsStorage.FindByName(link.Remote)
		if !ok {
			return nil, fmt.Errorf("port '%s' not found", link.Remote)
		}

		links = append(links, inventory.HWLink{
			Local:  inventory.IDName{Name: link.Local},
			Remote: inventory.IDName{ID: port.ID, Name: port.Port_},
		})
	}

	inventoryServerMeta := &k8sv1alpha1.InventoryServerMeta{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(inventoryServer.GetUID()),
			Namespace: inventoryServer.GetNamespace(),
		},
		TypeMeta: metav1.TypeMeta{},
		Spec: k8sv1alpha1.InventoryServerMetaSpec{
			Imported:            imported,
			Reclaim:             reclaim,
			InventoryServerName: inventoryServer.Name,
			Description:         inventoryServer.Spec.Description,
			TenantID:            tenantID,
			SiteID:              siteID,
			ASN:                 inventoryServer.Spec.ASN,
			ProfileID:           profileID,
			MainIP:              inventoryServer.Spec.MainIP,
			MgmtIP:              inventoryServer.Spec.MgmtIP,
			PortsCount:          inventoryServer.Spec.PortsCount,
			UUID:                inventoryServer.Spec.UUID,
			Links:               links,
			CustomData:          inventoryServer.Spec.CustomData,
			Tags:                inventoryServer.Spec.Tags,
			SRVRole:             inventoryServer.Spec.SRVRole,
		},
	}

	return inventoryServerMeta, nil
}

func inventoryServerCompareFieldsForNewMeta(inventoryServer *k8sv1alpha1.InventoryServer, inventoryServerMeta *k8sv1alpha1.InventoryServerMeta) bool {
	imported := false
	reclaim := false
	if i, ok := inventoryServer.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = true
	}
	if i, ok := inventoryServer.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = true
	}
	return inventoryServer.GetGeneration() != inventoryServerMeta.Spec.InventoryServerCRGeneration || imported != inventoryServerMeta.Spec.Imported || reclaim != inventoryServerMeta.Spec.Reclaim
}

func inventoryServerMustUpdateAnnotations(inventoryServer *k8sv1alpha1.InventoryServer) bool {
	update := false
	if i, ok := inventoryServer.GetAnnotations()["resource.k8s.netris.ai/import"]; !(ok && (i == "true" || i == "false")) {
		update = true
	}
	if i, ok := inventoryServer.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; !(ok && (i == "retain" || i == "delete")) {
		update = true
	}
	return update
}

func inventoryServerUpdateDefaultAnnotations(inventoryServer *k8sv1alpha1.InventoryServer) {
	imported := "false"
	reclaim := "delete"
	if i, ok := inventoryServer.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = "true"
	}
	if i, ok := inventoryServer.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = "retain"
	}
	annotations := inventoryServer.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["resource.k8s.netris.ai/import"] = imported
	annotations["resource.k8s.netris.ai/reclaimPolicy"] = reclaim
	inventoryServer.SetAnnotations(annotations)
}

// InventoryServerMetaToNetris converts the k8s InventoryServer resource to Netris type for add API call.
func InventoryServerMetaToNetris(inventoryServerMeta *k8sv1alpha1.InventoryServerMeta) (*inventory.HWServer, error) {
	mainIP := inventoryServerMeta.Spec.MainIP
	if inventoryServerMeta.Spec.MainIP == "" {
		mainIP = "auto"
	}

	mgmtIP := inventoryServerMeta.Spec.MgmtIP
	if inventoryServerMeta.Spec.MgmtIP == "" {
		mgmtIP = "auto"
	}

	var asn interface{} = inventoryServerMeta.Spec.ASN
	if inventoryServerMeta.Spec.ASN == 0 {
		asn = "auto"
	}

	serverAdd := &inventory.HWServer{
		Name:        inventoryServerMeta.Spec.InventoryServerName,
		Description: inventoryServerMeta.Spec.Description,
		Tenant:      inventory.IDName{ID: inventoryServerMeta.Spec.TenantID},
		Site:        inventory.IDName{ID: inventoryServerMeta.Spec.SiteID},
		Profile:     inventory.IDName{ID: inventoryServerMeta.Spec.ProfileID},
		Asn:         asn,
		MainAddress: mainIP,
		MgmtAddress: mgmtIP,
		PortCount:   inventoryServerMeta.Spec.PortsCount,
		UUID:        inventoryServerMeta.Spec.UUID,
		Links:       inventoryServerMeta.Spec.Links,
		CustomData:  inventoryServerMeta.Spec.CustomData,
		Tags:        inventoryServerMeta.Spec.Tags,
		SRVRole:     inventoryServerMeta.Spec.SRVRole,
	}

	return serverAdd, nil
}

// InventoryServerMetaToNetrisUpdate converts the k8s InventoryServer resource to Netris type for update API call.
func InventoryServerMetaToNetrisUpdate(inventoryServerMeta *k8sv1alpha1.InventoryServerMeta) (*inventory.HWServer, error) {
	mainIP := inventoryServerMeta.Spec.MainIP
	if inventoryServerMeta.Spec.MainIP == "" {
		mainIP = "auto"
	}

	mgmtIP := inventoryServerMeta.Spec.MgmtIP
	if inventoryServerMeta.Spec.MgmtIP == "" {
		mgmtIP = "auto"
	}

	var asn interface{} = inventoryServerMeta.Spec.ASN
	if inventoryServerMeta.Spec.ASN == 0 {
		asn = "auto"
	}

	serverUpdate := &inventory.HWServer{
		Name:        inventoryServerMeta.Spec.InventoryServerName,
		Description: inventoryServerMeta.Spec.Description,
		Tenant:      inventory.IDName{ID: inventoryServerMeta.Spec.TenantID},
		Site:        inventory.IDName{ID: inventoryServerMeta.Spec.SiteID},
		Profile:     inventory.IDName{ID: inventoryServerMeta.Spec.ProfileID},
		Asn:         asn,
		MainAddress: mainIP,
		MgmtAddress: mgmtIP,
		PortCount:   inventoryServerMeta.Spec.PortsCount,
		UUID:        inventoryServerMeta.Spec.UUID,
		Links:       inventoryServerMeta.Spec.Links,
		CustomData:  inventoryServerMeta.Spec.CustomData,
		Tags:        inventoryServerMeta.Spec.Tags,
		SRVRole:     inventoryServerMeta.Spec.SRVRole,
	}

	return serverUpdate, nil
}

func compareInventoryServerMetaAPIServer(inventoryServerMeta *k8sv1alpha1.InventoryServerMeta, apiServer *inventory.HW, u uniReconciler) bool {
	if apiServer.Name != inventoryServerMeta.Spec.InventoryServerName {
		u.DebugLogger.Info("Name changed", "netrisValue", apiServer.Name, "k8sValue", inventoryServerMeta.Spec.InventoryServerName)
		return false
	}

	if apiServer.Description != inventoryServerMeta.Spec.Description {
		u.DebugLogger.Info("Description changed", "netrisValue", apiServer.Description, "k8sValue", inventoryServerMeta.Spec.Description)
		return false
	}

	if apiServer.Tenant.ID != inventoryServerMeta.Spec.TenantID {
		u.DebugLogger.Info("Tenant changed", "netrisValue", apiServer.Tenant.ID, "k8sValue", inventoryServerMeta.Spec.TenantID)
		return false
	}

	if apiServer.Site.ID != inventoryServerMeta.Spec.SiteID {
		u.DebugLogger.Info("Site changed", "netrisValue", apiServer.Site.ID, "k8sValue", inventoryServerMeta.Spec.SiteID)
		return false
	}

	if apiServer.Profile.ID != inventoryServerMeta.Spec.ProfileID {
		u.DebugLogger.Info("Profile changed", "netrisValue", apiServer.Profile.ID, "k8sValue", inventoryServerMeta.Spec.ProfileID)
		return false
	}

	if apiServer.MainIP.Address != inventoryServerMeta.Spec.MainIP {
		u.DebugLogger.Info("MainIP changed", "netrisValue", apiServer.MainIP.Address, "k8sValue", inventoryServerMeta.Spec.MainIP)
		return false
	}

	if apiServer.MgmtIP.Address != inventoryServerMeta.Spec.MgmtIP {
		u.DebugLogger.Info("MgmtIP changed", "netrisValue", apiServer.MgmtIP.Address, "k8sValue", inventoryServerMeta.Spec.MgmtIP)
		return false
	}

	if apiServer.PortCount != inventoryServerMeta.Spec.PortsCount {
		u.DebugLogger.Info("PortsCount changed", "netrisValue", apiServer.PortCount, "k8sValue", inventoryServerMeta.Spec.PortsCount)
		return false
	}

	if apiServer.UUID != inventoryServerMeta.Spec.UUID {
		u.DebugLogger.Info("UUID changed", "netrisValue", apiServer.UUID, "k8sValue", inventoryServerMeta.Spec.UUID)
		return false
	}

	if apiServer.CustomData != inventoryServerMeta.Spec.CustomData {
		u.DebugLogger.Info("CustomData changed", "netrisValue", apiServer.CustomData, "k8sValue", inventoryServerMeta.Spec.CustomData)
		return false
	}

	if apiServer.SRVRole != inventoryServerMeta.Spec.SRVRole {
		u.DebugLogger.Info("SRVRole changed", "netrisValue", apiServer.SRVRole, "k8sValue", inventoryServerMeta.Spec.SRVRole)
		return false
	}

	return true
}
