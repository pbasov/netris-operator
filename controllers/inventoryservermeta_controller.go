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
	"github.com/netrisai/netriswebapi/v2/types/inventory"
)

// InventoryServerMetaReconciler reconciles a InventoryServerMeta object
type InventoryServerMetaReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Cred     *api.Clientset
	NStorage *netrisstorage.Storage
}

//+kubebuilder:rbac:groups=k8s.netris.ai,resources=inventoryservermeta,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=inventoryservermeta/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=inventoryservermeta/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InventoryServerMetaReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	debugLogger := r.Log.WithValues("name", req.NamespacedName).V(int(zapcore.WarnLevel))

	inventoryServerMeta := &k8sv1alpha1.InventoryServerMeta{}
	inventoryServerCR := &k8sv1alpha1.InventoryServer{}
	inventoryServerMetaCtx, inventoryServerMetaCancel := context.WithTimeout(cntxt, contextTimeout)
	defer inventoryServerMetaCancel()
	if err := r.Get(inventoryServerMetaCtx, req.NamespacedName, inventoryServerMeta); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger := r.Log.WithValues("name", fmt.Sprintf("%s/%s", req.NamespacedName.Namespace, inventoryServerMeta.Spec.InventoryServerName))
	debugLogger = logger.V(int(zapcore.WarnLevel))

	u := uniReconciler{
		Client:      r.Client,
		Logger:      logger,
		DebugLogger: debugLogger,
		Cred:        r.Cred,
		NStorage:    r.NStorage,
	}

	provisionState := "OK"

	inventoryServerNN := req.NamespacedName
	inventoryServerNN.Name = inventoryServerMeta.Spec.InventoryServerName
	inventoryServerNNCtx, inventoryServerNNCancel := context.WithTimeout(cntxt, contextTimeout)
	defer inventoryServerNNCancel()
	if err := r.Get(inventoryServerNNCtx, inventoryServerNN, inventoryServerCR); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if inventoryServerMeta.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	if inventoryServerMeta.Spec.ID == 0 {
		debugLogger.Info("ID Not found in meta")
		if inventoryServerMeta.Spec.Imported {
			logger.Info("Importing inventory server")
			debugLogger.Info("Imported yaml mode. Finding InventoryServer by name")
			if serverH, ok := r.NStorage.HWsStorage.FindServerByName(inventoryServerMeta.Spec.InventoryServerName); ok {
				debugLogger.Info("Imported yaml mode. InventoryServer found")
				inventoryServerMeta.Spec.ID = serverH.ID
				inventoryServerMeta.Spec.MainIP = serverH.MainIP.Address
				inventoryServerMeta.Spec.MgmtIP = serverH.MgmtIP.Address
				inventoryServerMeta.Spec.ASN = serverH.Asn

				inventoryServerMetaPatchCtx, inventoryServerMetaPatchCancel := context.WithTimeout(cntxt, contextTimeout)
				defer inventoryServerMetaPatchCancel()
				err := r.Patch(inventoryServerMetaPatchCtx, inventoryServerMeta.DeepCopyObject(), client.Merge, &client.PatchOptions{})
				if err != nil {
					logger.Error(fmt.Errorf("{patch inventoryServerMeta.Spec.ID} %s", err), "")
					return u.patchInventoryServerStatus(inventoryServerCR, "Failure", err.Error())
				}
				debugLogger.Info("Imported yaml mode. ID patched")
				logger.Info("InventoryServer imported")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
			logger.Info("InventoryServer not found for import")
			debugLogger.Info("Imported yaml mode. InventoryServer not found")
		}

		logger.Info("Creating InventoryServer")
		if _, err, errMsg := r.createInventoryServer(inventoryServerMeta); err != nil {
			logger.Error(fmt.Errorf("{createInventoryServer} %s", err), "")
			return u.patchInventoryServerStatus(inventoryServerCR, "Failure", errMsg.Error())
		}
		logger.Info("InventoryServer Created")
	} else {
		if apiServer, ok := r.NStorage.HWsStorage.FindServerByID(inventoryServerMeta.Spec.ID); ok {
			debugLogger.Info("Comparing InventoryServerMeta with Netris InventoryServer")

			if inventoryServerMeta.Spec.MainIP == "" {
				inventoryServerMeta.Spec.MainIP = apiServer.MainIP.Address
			}
			if inventoryServerMeta.Spec.MgmtIP == "" {
				inventoryServerMeta.Spec.MgmtIP = apiServer.MgmtIP.Address
			}
			if inventoryServerMeta.Spec.ASN == 0 {
				inventoryServerMeta.Spec.ASN = apiServer.Asn
			}

			if ok := compareInventoryServerMetaAPIServer(inventoryServerMeta, apiServer, u); ok {
				debugLogger.Info("Nothing Changed")
			} else {
				debugLogger.Info("Go to update InventoryServer in Netris")
				logger.Info("Updating InventoryServer")
				serverUpdate, err := InventoryServerMetaToNetrisUpdate(inventoryServerMeta)
				if err != nil {
					logger.Error(fmt.Errorf("{InventoryServerMetaToNetrisUpdate} %s", err), "")
					return u.patchInventoryServerStatus(inventoryServerCR, "Failure", err.Error())
				}

				js, _ := json.Marshal(serverUpdate)
				debugLogger.Info("serverUpdate", "payload", string(js))

				_, err, errMsg := updateInventoryServer(inventoryServerMeta.Spec.ID, serverUpdate, r.Cred)
				if err != nil {
					logger.Error(fmt.Errorf("{updateInventoryServer} %s", err), "")
					return u.patchInventoryServerStatus(inventoryServerCR, "Failure", errMsg.Error())
				}
				logger.Info("InventoryServer Updated")
			}
		} else {
			debugLogger.Info("InventoryServer not found in Netris")
			debugLogger.Info("Going to create InventoryServer")
			logger.Info("Creating InventoryServer")
			if _, err, errMsg := r.createInventoryServer(inventoryServerMeta); err != nil {
				logger.Error(fmt.Errorf("{createInventoryServer} %s", err), "")
				return u.patchInventoryServerStatus(inventoryServerCR, "Failure", errMsg.Error())
			}
			logger.Info("InventoryServer Created")
		}
	}

	if _, err := u.updateInventoryServerIfNecessary(inventoryServerCR, *inventoryServerMeta); err != nil {
		logger.Error(fmt.Errorf("{updateInventoryServerIfNecessary} %s", err), "")
		return u.patchInventoryServerStatus(inventoryServerCR, "Failure", err.Error())
	}

	return u.patchInventoryServerStatus(inventoryServerCR, provisionState, "Success")
}

func (r *InventoryServerMetaReconciler) createInventoryServer(inventoryServerMeta *k8sv1alpha1.InventoryServerMeta) (ctrl.Result, error, error) {
	debugLogger := r.Log.WithValues(
		"name", fmt.Sprintf("%s/%s", inventoryServerMeta.Namespace, inventoryServerMeta.Spec.InventoryServerName),
		"inventoryServerName", inventoryServerMeta.Spec.InventoryServerCRGeneration,
	).V(int(zapcore.WarnLevel))

	serverAdd, err := InventoryServerMetaToNetris(inventoryServerMeta)
	if err != nil {
		return ctrl.Result{}, err, err
	}

	js, _ := json.Marshal(serverAdd)
	debugLogger.Info("serverToAdd", "payload", string(js))

	reply, err := r.Cred.Inventory().AddServer(serverAdd)
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

	debugLogger.Info("InventoryServer Created", "id", idStruct.ID)

	inventoryServerMeta.Spec.ID = idStruct.ID

	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	err = r.Patch(ctx, inventoryServerMeta.DeepCopyObject(), client.Merge, &client.PatchOptions{})
	if err != nil {
		return ctrl.Result{}, err, err
	}

	debugLogger.Info("ID patched to meta", "id", idStruct.ID)
	return ctrl.Result{}, nil, nil
}

func updateInventoryServer(id int, serverH *inventory.HWServer, cred *api.Clientset) (ctrl.Result, error, error) {
	reply, err := cred.Inventory().UpdateServer(id, serverH)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("{updateInventoryServer} %s", err), err
	}
	resp, err := http.ParseAPIResponse(reply.Data)
	if err != nil {
		return ctrl.Result{}, err, err
	}
	if !resp.IsSuccess {
		return ctrl.Result{}, fmt.Errorf("{updateInventoryServer} %s", fmt.Errorf(resp.Message)), fmt.Errorf(resp.Message)
	}

	return ctrl.Result{}, nil, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InventoryServerMetaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.InventoryServerMeta{}).
		Complete(r)
}

func (u *uniReconciler) updateInventoryServerIfNecessary(inventoryServerCR *k8sv1alpha1.InventoryServer, inventoryServerMeta k8sv1alpha1.InventoryServerMeta) (ctrl.Result, error) {
	shouldUpdateCR := false
	if inventoryServerCR.Spec.MainIP == "" && inventoryServerCR.Spec.MainIP != inventoryServerMeta.Spec.MainIP {
		inventoryServerCR.Spec.MainIP = inventoryServerMeta.Spec.MainIP
		shouldUpdateCR = true
	}
	if inventoryServerCR.Spec.MgmtIP == "" && inventoryServerCR.Spec.MgmtIP != inventoryServerMeta.Spec.MgmtIP {
		inventoryServerCR.Spec.MgmtIP = inventoryServerMeta.Spec.MgmtIP
		shouldUpdateCR = true
	}
	if inventoryServerCR.Spec.ASN == 0 && inventoryServerCR.Spec.ASN != inventoryServerMeta.Spec.ASN {
		inventoryServerCR.Spec.ASN = inventoryServerMeta.Spec.ASN
		shouldUpdateCR = true
	}
	if shouldUpdateCR {
		u.DebugLogger.Info("Updating InventoryServer CR")
		if _, err := u.patchInventoryServer(inventoryServerCR); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}
