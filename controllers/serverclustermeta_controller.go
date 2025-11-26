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
	"github.com/netrisai/netriswebapi/v2/types/servercluster"
)

// ServerClusterMetaReconciler reconciles a ServerClusterMeta object
type ServerClusterMetaReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Cred     *api.Clientset
	NStorage *netrisstorage.Storage
}

//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclustersmeta,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclustersmeta/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclustersmeta/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ServerClusterMetaReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	debugLogger := r.Log.WithValues("name", req.NamespacedName).V(int(zapcore.WarnLevel))

	clusterMeta := &k8sv1alpha1.ServerClusterMeta{}
	clusterCR := &k8sv1alpha1.ServerCluster{}
	clusterMetaCtx, clusterMetaCancel := context.WithTimeout(cntxt, contextTimeout)
	defer clusterMetaCancel()
	if err := r.Get(clusterMetaCtx, req.NamespacedName, clusterMeta); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger := r.Log.WithValues("name", fmt.Sprintf("%s/%s", req.NamespacedName.Namespace, clusterMeta.Spec.ServerClusterName))
	debugLogger = logger.V(int(zapcore.WarnLevel))

	u := uniReconciler{
		Client:      r.Client,
		Logger:      logger,
		DebugLogger: debugLogger,
		Cred:        r.Cred,
		NStorage:    r.NStorage,
	}

	provisionState := "OK"

	clusterNN := req.NamespacedName
	clusterNN.Name = clusterMeta.Spec.ServerClusterName
	clusterNNCtx, clusterNNCancel := context.WithTimeout(cntxt, contextTimeout)
	defer clusterNNCancel()
	if err := r.Get(clusterNNCtx, clusterNN, clusterCR); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if clusterMeta.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	if clusterMeta.Spec.ID == 0 {
		debugLogger.Info("ID Not found in meta")
		if clusterMeta.Spec.Imported {
			logger.Info("Importing server cluster")
			debugLogger.Info("Imported yaml mode. Finding ServerCluster by name")
			if apiCluster, ok := r.NStorage.ServerClusterStorage.FindByName(clusterMeta.Spec.ServerClusterName); ok {
				debugLogger.Info("Imported yaml mode. ServerCluster found")
				clusterMeta.Spec.ID = apiCluster.ID

				clusterMetaPatchCtx, clusterMetaPatchCancel := context.WithTimeout(cntxt, contextTimeout)
				defer clusterMetaPatchCancel()
				err := r.Patch(clusterMetaPatchCtx, clusterMeta.DeepCopyObject(), client.Merge, &client.PatchOptions{})
				if err != nil {
					logger.Error(fmt.Errorf("{patch clusterMeta.Spec.ID} %s", err), "")
					return u.patchServerClusterStatus(clusterCR, "Failure", err.Error())
				}
				debugLogger.Info("Imported yaml mode. ID patched")
				logger.Info("ServerCluster imported")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
			logger.Info("ServerCluster not found for import")
			debugLogger.Info("Imported yaml mode. ServerCluster not found")
		}

		logger.Info("Creating ServerCluster")
		if _, err, errMsg := r.createServerCluster(clusterMeta); err != nil {
			logger.Error(fmt.Errorf("{createServerCluster} %s", err), "")
			return u.patchServerClusterStatus(clusterCR, "Failure", errMsg.Error())
		}
		logger.Info("ServerCluster Created")
	} else {
		if apiCluster, ok := r.NStorage.ServerClusterStorage.FindByID(clusterMeta.Spec.ID); ok {
			debugLogger.Info("Comparing ServerClusterMeta with Netris ServerCluster")

			if ok := compareServerClusterMetaAPI(clusterMeta, apiCluster, u); ok {
				debugLogger.Info("Nothing Changed")
			} else {
				debugLogger.Info("Go to update ServerCluster in Netris")
				logger.Info("Updating ServerCluster")
				clusterUpdate := ServerClusterMetaToNetrisUpdate(clusterMeta)

				js, _ := json.Marshal(clusterUpdate)
				debugLogger.Info("clusterUpdate", "payload", string(js))

				_, err, errMsg := updateServerCluster(clusterMeta.Spec.ID, clusterUpdate, r.Cred)
				if err != nil {
					logger.Error(fmt.Errorf("{updateServerCluster} %s", err), "")
					return u.patchServerClusterStatus(clusterCR, "Failure", errMsg.Error())
				}
				logger.Info("ServerCluster Updated")
			}
		} else {
			debugLogger.Info("ServerCluster not found in Netris")
			debugLogger.Info("Going to create ServerCluster")
			logger.Info("Creating ServerCluster")
			if _, err, errMsg := r.createServerCluster(clusterMeta); err != nil {
				logger.Error(fmt.Errorf("{createServerCluster} %s", err), "")
				return u.patchServerClusterStatus(clusterCR, "Failure", errMsg.Error())
			}
			logger.Info("ServerCluster Created")
		}
	}

	return u.patchServerClusterStatus(clusterCR, provisionState, "Success")
}

func (r *ServerClusterMetaReconciler) createServerCluster(clusterMeta *k8sv1alpha1.ServerClusterMeta) (ctrl.Result, error, error) {
	debugLogger := r.Log.WithValues(
		"name", fmt.Sprintf("%s/%s", clusterMeta.Namespace, clusterMeta.Spec.ServerClusterName),
		"clusterName", clusterMeta.Spec.ServerClusterCRGeneration,
	).V(int(zapcore.WarnLevel))

	clusterAdd := ServerClusterMetaToNetris(clusterMeta)

	js, _ := json.Marshal(clusterAdd)
	debugLogger.Info("clusterToAdd", "payload", string(js))

	reply, err := r.Cred.ServerCluster().Add(clusterAdd)
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

	debugLogger.Info("ServerCluster Created", "id", idStruct.ID)

	clusterMeta.Spec.ID = idStruct.ID

	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	err = r.Patch(ctx, clusterMeta.DeepCopyObject(), client.Merge, &client.PatchOptions{})
	if err != nil {
		return ctrl.Result{}, err, err
	}

	debugLogger.Info("ID patched to meta", "id", idStruct.ID)
	return ctrl.Result{}, nil, nil
}

func updateServerCluster(id int, cluster *servercluster.ServerClusterU, cred *api.Clientset) (ctrl.Result, error, error) {
	reply, err := cred.ServerCluster().Update(id, cluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("{updateServerCluster} %s", err), err
	}
	resp, err := http.ParseAPIResponse(reply.Data)
	if err != nil {
		return ctrl.Result{}, err, err
	}
	if !resp.IsSuccess {
		return ctrl.Result{}, fmt.Errorf("{updateServerCluster} %s", fmt.Errorf(resp.Message)), fmt.Errorf(resp.Message)
	}

	return ctrl.Result{}, nil, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServerClusterMetaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.ServerClusterMeta{}).
		Complete(r)
}
