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
)

// InventoryServerReconciler reconciles a InventoryServer object
type InventoryServerReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Cred     *api.Clientset
	NStorage *netrisstorage.Storage
}

//+kubebuilder:rbac:groups=k8s.netris.ai,resources=inventoryservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=inventoryservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=inventoryservers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InventoryServerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("name", req.NamespacedName)
	debugLogger := logger.V(int(zapcore.WarnLevel))
	inventoryServer := &k8sv1alpha1.InventoryServer{}

	u := uniReconciler{
		Client:      r.Client,
		Logger:      logger,
		DebugLogger: debugLogger,
		Cred:        r.Cred,
		NStorage:    r.NStorage,
	}

	inventoryServerCtx, inventoryServerCancel := context.WithTimeout(cntxt, contextTimeout)
	defer inventoryServerCancel()
	if err := r.Get(inventoryServerCtx, req.NamespacedName, inventoryServer); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	inventoryServerMetaNamespaced := req.NamespacedName
	inventoryServerMetaNamespaced.Name = string(inventoryServer.GetUID())
	inventoryServerMeta := &k8sv1alpha1.InventoryServerMeta{}
	metaFound := true

	inventoryServerMetaCtx, inventoryServerMetaCancel := context.WithTimeout(cntxt, contextTimeout)
	defer inventoryServerMetaCancel()
	if err := r.Get(inventoryServerMetaCtx, inventoryServerMetaNamespaced, inventoryServerMeta); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			metaFound = false
			inventoryServerMeta = nil
		} else {
			return ctrl.Result{}, err
		}
	}

	if inventoryServer.DeletionTimestamp != nil {
		logger.Info("Go to delete")
		_, err := r.deleteInventoryServer(inventoryServer, inventoryServerMeta)
		if err != nil {
			logger.Error(fmt.Errorf("{deleteInventoryServer} %s", err), "")
			return u.patchInventoryServerStatus(inventoryServer, "Failure", err.Error())
		}
		logger.Info("InventoryServer deleted")
		return ctrl.Result{}, nil
	}

	if inventoryServerMustUpdateAnnotations(inventoryServer) {
		debugLogger.Info("Setting default annotations")
		inventoryServerUpdateDefaultAnnotations(inventoryServer)
		inventoryServerPatchCtx, inventoryServerPatchCancel := context.WithTimeout(cntxt, contextTimeout)
		defer inventoryServerPatchCancel()
		err := r.Patch(inventoryServerPatchCtx, inventoryServer.DeepCopyObject(), client.Merge, &client.PatchOptions{})
		if err != nil {
			logger.Error(fmt.Errorf("{Patch InventoryServer default annotations} %s", err), "")
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
		return ctrl.Result{}, nil
	}

	if metaFound {
		debugLogger.Info("Meta found")
		if inventoryServerCompareFieldsForNewMeta(inventoryServer, inventoryServerMeta) {
			debugLogger.Info("Generating New Meta")
			inventoryServerID := inventoryServerMeta.Spec.ID
			newInventoryServerMeta, err := r.InventoryServerToInventoryServerMeta(inventoryServer)
			if err != nil {
				logger.Error(fmt.Errorf("{InventoryServerToInventoryServerMeta} %s", err), "")
				return u.patchInventoryServerStatus(inventoryServer, "Failure", err.Error())
			}
			inventoryServerMeta.Spec = newInventoryServerMeta.DeepCopy().Spec
			inventoryServerMeta.Spec.ID = inventoryServerID
			inventoryServerMeta.Spec.InventoryServerCRGeneration = inventoryServer.GetGeneration()

			inventoryServerMetaUpdateCtx, inventoryServerMetaUpdateCancel := context.WithTimeout(cntxt, contextTimeout)
			defer inventoryServerMetaUpdateCancel()
			err = r.Update(inventoryServerMetaUpdateCtx, inventoryServerMeta.DeepCopyObject(), &client.UpdateOptions{})
			if err != nil {
				logger.Error(fmt.Errorf("{inventoryServerMeta Update} %s", err), "")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
		}
	} else {
		debugLogger.Info("Meta not found")
		if inventoryServer.GetFinalizers() == nil {
			inventoryServer.SetFinalizers([]string{"resource.k8s.netris.ai/delete"})

			inventoryServerPatchCtx, inventoryServerPatchCancel := context.WithTimeout(cntxt, contextTimeout)
			defer inventoryServerPatchCancel()
			err := r.Patch(inventoryServerPatchCtx, inventoryServer.DeepCopyObject(), client.Merge, &client.PatchOptions{})
			if err != nil {
				logger.Error(fmt.Errorf("{Patch InventoryServer Finalizer} %s", err), "")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
			return ctrl.Result{}, nil
		}

		inventoryServerMeta, err := r.InventoryServerToInventoryServerMeta(inventoryServer)
		if err != nil {
			logger.Error(fmt.Errorf("{InventoryServerToInventoryServerMeta} %s", err), "")
			return u.patchInventoryServerStatus(inventoryServer, "Failure", err.Error())
		}

		inventoryServerMeta.Spec.InventoryServerCRGeneration = inventoryServer.GetGeneration()

		inventoryServerMetaCreateCtx, inventoryServerMetaCreateCancel := context.WithTimeout(cntxt, contextTimeout)
		defer inventoryServerMetaCreateCancel()
		if err := r.Create(inventoryServerMetaCreateCtx, inventoryServerMeta.DeepCopyObject(), &client.CreateOptions{}); err != nil {
			logger.Error(fmt.Errorf("{inventoryServerMeta Create} %s", err), "")
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *InventoryServerReconciler) deleteInventoryServer(inventoryServer *k8sv1alpha1.InventoryServer, inventoryServerMeta *k8sv1alpha1.InventoryServerMeta) (ctrl.Result, error) {
	if inventoryServerMeta != nil && inventoryServerMeta.Spec.ID > 0 && !inventoryServerMeta.Spec.Reclaim {
		reply, err := r.Cred.Inventory().Delete("server", inventoryServerMeta.Spec.ID)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("{deleteInventoryServer} %s", err)
		}
		resp, err := http.ParseAPIResponse(reply.Data)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !resp.IsSuccess && resp.Meta.StatusCode != 404 {
			return ctrl.Result{}, fmt.Errorf("{deleteInventoryServer} %s", fmt.Errorf(resp.Message))
		}
	}
	return r.deleteCRs(inventoryServer, inventoryServerMeta)
}

func (r *InventoryServerReconciler) deleteCRs(inventoryServer *k8sv1alpha1.InventoryServer, inventoryServerMeta *k8sv1alpha1.InventoryServerMeta) (ctrl.Result, error) {
	if inventoryServerMeta != nil {
		_, err := r.deleteInventoryServerMetaCR(inventoryServerMeta)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("{deleteCRs} %s", err)
		}
	}

	return r.deleteInventoryServerCR(inventoryServer)
}

func (r *InventoryServerReconciler) deleteInventoryServerCR(inventoryServer *k8sv1alpha1.InventoryServer) (ctrl.Result, error) {
	inventoryServer.ObjectMeta.SetFinalizers(nil)
	inventoryServer.SetFinalizers(nil)
	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	if err := r.Update(ctx, inventoryServer.DeepCopyObject(), &client.UpdateOptions{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("{deleteInventoryServerCR} %s", err)
	}

	return ctrl.Result{}, nil
}

func (r *InventoryServerReconciler) deleteInventoryServerMetaCR(inventoryServerMeta *k8sv1alpha1.InventoryServerMeta) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	if err := r.Delete(ctx, inventoryServerMeta.DeepCopyObject(), &client.DeleteOptions{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("{deleteInventoryServerMetaCR} %s", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InventoryServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.InventoryServer{}).
		Complete(r)
}
