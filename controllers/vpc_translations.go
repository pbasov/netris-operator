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
	"github.com/netrisai/netriswebapi/v2/types/vpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VPCToMeta converts the VPC resource to Meta type.
func (r *VPCReconciler) VPCToMeta(vpcCR *k8sv1alpha1.VPC) (*k8sv1alpha1.VPCMeta, error) {
	var (
		imported = false
		reclaim  = false
	)

	if i, ok := vpcCR.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = true
	}
	if i, ok := vpcCR.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = true
	}

	adminTenantID := 0
	adminTenantName := vpcCR.Spec.AdminTenant
	if tenant, ok := r.NStorage.TenantsStorage.FindByName(vpcCR.Spec.AdminTenant); ok {
		adminTenantID = tenant.ID
		adminTenantName = tenant.Name
	} else {
		return nil, fmt.Errorf("couldn't find admin tenant '%s'", vpcCR.Spec.AdminTenant)
	}

	// Resolve guest tenants
	guestTenants := []k8sv1alpha1.VPCGuestTenant{}
	for _, tenantName := range vpcCR.Spec.GuestTenants {
		if tenant, ok := r.NStorage.TenantsStorage.FindByName(tenantName); ok {
			guestTenants = append(guestTenants, k8sv1alpha1.VPCGuestTenant{
				ID:   tenant.ID,
				Name: tenant.Name,
			})
		} else {
			return nil, fmt.Errorf("couldn't find guest tenant '%s'", tenantName)
		}
	}

	vpcMeta := &k8sv1alpha1.VPCMeta{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(vpcCR.GetUID()),
			Namespace: vpcCR.GetNamespace(),
		},
		TypeMeta: metav1.TypeMeta{},
		Spec: k8sv1alpha1.VPCMetaSpec{
			Imported:        imported,
			Reclaim:         reclaim,
			VPCName:         vpcCR.Name,
			AdminTenantID:   adminTenantID,
			AdminTenantName: adminTenantName,
			GuestTenants:    guestTenants,
			Tags:            vpcCR.Spec.Tags,
		},
	}

	return vpcMeta, nil
}

func vpcCompareFieldsForNewMeta(vpcCR *k8sv1alpha1.VPC, vpcMeta *k8sv1alpha1.VPCMeta) bool {
	imported := false
	reclaim := false
	if i, ok := vpcCR.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = true
	}
	if i, ok := vpcCR.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = true
	}
	return vpcCR.GetGeneration() != vpcMeta.Spec.VPCCRGeneration || imported != vpcMeta.Spec.Imported || reclaim != vpcMeta.Spec.Reclaim
}

func vpcMustUpdateAnnotations(vpcCR *k8sv1alpha1.VPC) bool {
	update := false
	if i, ok := vpcCR.GetAnnotations()["resource.k8s.netris.ai/import"]; !(ok && (i == "true" || i == "false")) {
		update = true
	}
	if i, ok := vpcCR.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; !(ok && (i == "retain" || i == "delete")) {
		update = true
	}
	return update
}

func vpcUpdateDefaultAnnotations(vpcCR *k8sv1alpha1.VPC) {
	imported := "false"
	reclaim := "delete"
	if i, ok := vpcCR.GetAnnotations()["resource.k8s.netris.ai/import"]; ok && i == "true" {
		imported = "true"
	}
	if i, ok := vpcCR.GetAnnotations()["resource.k8s.netris.ai/reclaimPolicy"]; ok && i == "retain" {
		reclaim = "retain"
	}
	annotations := vpcCR.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["resource.k8s.netris.ai/import"] = imported
	annotations["resource.k8s.netris.ai/reclaimPolicy"] = reclaim
	vpcCR.SetAnnotations(annotations)
}

// VPCMetaToNetris converts Meta to Netris API type.
func VPCMetaToNetris(vpcMeta *k8sv1alpha1.VPCMeta) *vpc.VPCw {
	guestTenants := []vpc.GuestTenant{}
	for _, guest := range vpcMeta.Spec.GuestTenants {
		guestTenants = append(guestTenants, vpc.GuestTenant{
			ID:   guest.ID,
			Name: guest.Name,
		})
	}

	return &vpc.VPCw{
		Name: vpcMeta.Spec.VPCName,
		AdminTenant: vpc.AdminTenant{
			ID:   vpcMeta.Spec.AdminTenantID,
			Name: vpcMeta.Spec.AdminTenantName,
		},
		GuestTenant: guestTenants,
		Tags:        vpcMeta.Spec.Tags,
	}
}

func compareVPCMetaAPI(vpcMeta *k8sv1alpha1.VPCMeta, apiVPC *vpc.VPC, u uniReconciler) bool {
	if apiVPC.Name != vpcMeta.Spec.VPCName {
		u.DebugLogger.Info("Name changed", "netrisValue", apiVPC.Name, "k8sValue", vpcMeta.Spec.VPCName)
		return false
	}
	if apiVPC.AdminTenant.ID != vpcMeta.Spec.AdminTenantID {
		u.DebugLogger.Info("AdminTenantID changed", "netrisValue", apiVPC.AdminTenant.ID, "k8sValue", vpcMeta.Spec.AdminTenantID)
		return false
	}
	if len(apiVPC.GuestTenant) != len(vpcMeta.Spec.GuestTenants) {
		u.DebugLogger.Info("GuestTenants count changed", "netrisValue", len(apiVPC.GuestTenant), "k8sValue", len(vpcMeta.Spec.GuestTenants))
		return false
	}
	return true
}
