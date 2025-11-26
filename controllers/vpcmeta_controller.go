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
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"

	k8sv1alpha1 "github.com/netrisai/netris-operator/api/v1alpha1"
	"github.com/netrisai/netris-operator/netrisstorage"
	"github.com/netrisai/netriswebapi/http"
	api "github.com/netrisai/netriswebapi/v2"
	"github.com/netrisai/netriswebapi/v2/types/vpc"
)

// VPCMetaReconciler reconciles a VPCMeta object
type VPCMetaReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Cred     *api.Clientset
	NStorage *netrisstorage.Storage
}

//+kubebuilder:rbac:groups=k8s.netris.ai,resources=vpcsmeta,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=vpcsmeta/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=vpcsmeta/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *VPCMetaReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	debugLogger := r.Log.WithValues("name", req.NamespacedName).V(int(zapcore.WarnLevel))

	vpcMeta := &k8sv1alpha1.VPCMeta{}
	vpcCR := &k8sv1alpha1.VPC{}
	vpcMetaCtx, vpcMetaCancel := context.WithTimeout(cntxt, contextTimeout)
	defer vpcMetaCancel()
	if err := r.Get(vpcMetaCtx, req.NamespacedName, vpcMeta); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger := r.Log.WithValues("name", fmt.Sprintf("%s/%s", req.NamespacedName.Namespace, vpcMeta.Spec.VPCName))
	debugLogger = logger.V(int(zapcore.WarnLevel))

	u := uniReconciler{
		Client:      r.Client,
		Logger:      logger,
		DebugLogger: debugLogger,
		Cred:        r.Cred,
		NStorage:    r.NStorage,
	}

	provisionState := "OK"

	vpcNN := req.NamespacedName
	vpcNN.Name = vpcMeta.Spec.VPCName
	vpcNNCtx, vpcNNCancel := context.WithTimeout(cntxt, contextTimeout)
	defer vpcNNCancel()
	if err := r.Get(vpcNNCtx, vpcNN, vpcCR); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if vpcMeta.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	if vpcMeta.Spec.ID == 0 {
		debugLogger.Info("ID Not found in meta")
		if vpcMeta.Spec.Imported {
			logger.Info("Importing VPC")
			debugLogger.Info("Imported yaml mode. Finding VPC by name")
			if apiVPC, ok := r.NStorage.VPCStorage.FindByName(vpcMeta.Spec.VPCName); ok {
				debugLogger.Info("Imported yaml mode. VPC found")
				vpcMeta.Spec.ID = apiVPC.ID

				vpcMetaPatchCtx, vpcMetaPatchCancel := context.WithTimeout(cntxt, contextTimeout)
				defer vpcMetaPatchCancel()
				err := r.Patch(vpcMetaPatchCtx, vpcMeta.DeepCopyObject(), client.Merge, &client.PatchOptions{})
				if err != nil {
					logger.Error(fmt.Errorf("{patch vpcMeta.Spec.ID} %s", err), "")
					return u.patchVPCStatus(vpcCR, "Failure", err.Error())
				}
				debugLogger.Info("Imported yaml mode. ID patched")
				logger.Info("VPC imported")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
			logger.Info("VPC not found for import")
			debugLogger.Info("Imported yaml mode. VPC not found")
		}

		logger.Info("Creating VPC")
		if _, err, errMsg := r.createVPC(vpcMeta); err != nil {
			logger.Error(fmt.Errorf("{createVPC} %s", err), "")
			return u.patchVPCStatus(vpcCR, "Failure", errMsg.Error())
		}
		logger.Info("VPC Created")
	} else {
		if apiVPC, ok := r.NStorage.VPCStorage.FindByID(vpcMeta.Spec.ID); ok {
			debugLogger.Info("Comparing VPCMeta with Netris VPC")

			if ok := compareVPCMetaAPI(vpcMeta, apiVPC, u); ok {
				debugLogger.Info("Nothing Changed")
			} else {
				debugLogger.Info("Go to update VPC in Netris")
				logger.Info("Updating VPC")
				vpcUpdate := VPCMetaToNetris(vpcMeta)

				js, _ := json.Marshal(vpcUpdate)
				debugLogger.Info("vpcUpdate", "payload", string(js))

				_, err, errMsg := updateVPC(vpcMeta.Spec.ID, vpcUpdate, r.Cred)
				if err != nil {
					logger.Error(fmt.Errorf("{updateVPC} %s", err), "")
					return u.patchVPCStatus(vpcCR, "Failure", errMsg.Error())
				}
				logger.Info("VPC Updated")
			}
		} else {
			debugLogger.Info("VPC not found in Netris")
			debugLogger.Info("Going to create VPC")
			logger.Info("Creating VPC")
			if _, err, errMsg := r.createVPC(vpcMeta); err != nil {
				logger.Error(fmt.Errorf("{createVPC} %s", err), "")
				return u.patchVPCStatus(vpcCR, "Failure", errMsg.Error())
			}
			logger.Info("VPC Created")
		}
	}

	return u.patchVPCStatus(vpcCR, provisionState, "Success")
}

func (r *VPCMetaReconciler) createVPC(vpcMeta *k8sv1alpha1.VPCMeta) (ctrl.Result, error, error) {
	debugLogger := r.Log.WithValues(
		"name", fmt.Sprintf("%s/%s", vpcMeta.Namespace, vpcMeta.Spec.VPCName),
		"vpcName", vpcMeta.Spec.VPCCRGeneration,
	).V(int(zapcore.WarnLevel))

	vpcAdd := VPCMetaToNetris(vpcMeta)

	js, _ := json.Marshal(vpcAdd)
	debugLogger.Info("vpcToAdd", "payload", string(js))

	reply, err := r.Cred.VPC().Add(vpcAdd)
	if err != nil {
		return ctrl.Result{}, err, err
	}

	idStruct := struct {
		ID int `json:"id"`
	}{}

	data, err := reply.Parse()
	if err != nil {
		return ctrl.Result{}, err, err
	}

	if reply.StatusCode != 200 {
		return ctrl.Result{}, fmt.Errorf(data.Message), fmt.Errorf(data.Message)
	}

	idStruct.ID = int(data.Data.(map[string]interface{})["id"].(float64))

	debugLogger.Info("VPC Created", "id", idStruct.ID)

	vpcMeta.Spec.ID = idStruct.ID

	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	err = r.Patch(ctx, vpcMeta.DeepCopyObject(), client.Merge, &client.PatchOptions{})
	if err != nil {
		return ctrl.Result{}, err, err
	}

	debugLogger.Info("ID patched to meta", "id", idStruct.ID)
	return ctrl.Result{}, nil, nil
}

func updateVPC(id int, vpcW *vpc.VPCw, cred *api.Clientset) (ctrl.Result, error, error) {
	reply, err := cred.VPC().Update(id, vpcW)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("{updateVPC} %s", err), err
	}
	resp, err := http.ParseAPIResponse(reply.Data)
	if err != nil {
		return ctrl.Result{}, err, err
	}
	if !resp.IsSuccess {
		return ctrl.Result{}, fmt.Errorf("{updateVPC} %s", fmt.Errorf(resp.Message)), fmt.Errorf(resp.Message)
	}

	return ctrl.Result{}, nil, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VPCMetaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.VPCMeta{}).
		Complete(r)
}
