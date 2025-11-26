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

// ServerClusterReconciler reconciles a ServerCluster object
type ServerClusterReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Cred     *api.Clientset
	NStorage *netrisstorage.Storage
}

//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ServerClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("name", req.NamespacedName)
	debugLogger := logger.V(int(zapcore.WarnLevel))
	cluster := &k8sv1alpha1.ServerCluster{}

	u := uniReconciler{
		Client:      r.Client,
		Logger:      logger,
		DebugLogger: debugLogger,
		Cred:        r.Cred,
		NStorage:    r.NStorage,
	}

	clusterCtx, clusterCancel := context.WithTimeout(cntxt, contextTimeout)
	defer clusterCancel()
	if err := r.Get(clusterCtx, req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	clusterMetaNamespaced := req.NamespacedName
	clusterMetaNamespaced.Name = string(cluster.GetUID())
	clusterMeta := &k8sv1alpha1.ServerClusterMeta{}
	metaFound := true

	clusterMetaCtx, clusterMetaCancel := context.WithTimeout(cntxt, contextTimeout)
	defer clusterMetaCancel()
	if err := r.Get(clusterMetaCtx, clusterMetaNamespaced, clusterMeta); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			metaFound = false
			clusterMeta = nil
		} else {
			return ctrl.Result{}, err
		}
	}

	if cluster.DeletionTimestamp != nil {
		logger.Info("Go to delete")
		_, err := r.deleteServerCluster(cluster, clusterMeta)
		if err != nil {
			logger.Error(fmt.Errorf("{deleteServerCluster} %s", err), "")
			return u.patchServerClusterStatus(cluster, "Failure", err.Error())
		}
		logger.Info("ServerCluster deleted")
		return ctrl.Result{}, nil
	}

	if serverClusterMustUpdateAnnotations(cluster) {
		debugLogger.Info("Setting default annotations")
		serverClusterUpdateDefaultAnnotations(cluster)
		clusterPatchCtx, clusterPatchCancel := context.WithTimeout(cntxt, contextTimeout)
		defer clusterPatchCancel()
		err := r.Patch(clusterPatchCtx, cluster.DeepCopyObject(), client.Merge, &client.PatchOptions{})
		if err != nil {
			logger.Error(fmt.Errorf("{Patch ServerCluster default annotations} %s", err), "")
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
		return ctrl.Result{}, nil
	}

	if metaFound {
		debugLogger.Info("Meta found")
		if serverClusterCompareFieldsForNewMeta(cluster, clusterMeta) {
			debugLogger.Info("Generating New Meta")
			clusterID := clusterMeta.Spec.ID
			newClusterMeta, err := r.ServerClusterToMeta(cluster)
			if err != nil {
				logger.Error(fmt.Errorf("{ServerClusterToMeta} %s", err), "")
				return u.patchServerClusterStatus(cluster, "Failure", err.Error())
			}
			clusterMeta.Spec = newClusterMeta.DeepCopy().Spec
			clusterMeta.Spec.ID = clusterID
			clusterMeta.Spec.ServerClusterCRGeneration = cluster.GetGeneration()

			clusterMetaUpdateCtx, clusterMetaUpdateCancel := context.WithTimeout(cntxt, contextTimeout)
			defer clusterMetaUpdateCancel()
			err = r.Update(clusterMetaUpdateCtx, clusterMeta.DeepCopyObject(), &client.UpdateOptions{})
			if err != nil {
				logger.Error(fmt.Errorf("{clusterMeta Update} %s", err), "")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
		}
	} else {
		debugLogger.Info("Meta not found")
		if cluster.GetFinalizers() == nil {
			cluster.SetFinalizers([]string{"resource.k8s.netris.ai/delete"})

			clusterPatchCtx, clusterPatchCancel := context.WithTimeout(cntxt, contextTimeout)
			defer clusterPatchCancel()
			err := r.Patch(clusterPatchCtx, cluster.DeepCopyObject(), client.Merge, &client.PatchOptions{})
			if err != nil {
				logger.Error(fmt.Errorf("{Patch ServerCluster Finalizer} %s", err), "")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
			return ctrl.Result{}, nil
		}

		clusterMeta, err := r.ServerClusterToMeta(cluster)
		if err != nil {
			logger.Error(fmt.Errorf("{ServerClusterToMeta} %s", err), "")
			return u.patchServerClusterStatus(cluster, "Failure", err.Error())
		}

		clusterMeta.Spec.ServerClusterCRGeneration = cluster.GetGeneration()

		clusterMetaCreateCtx, clusterMetaCreateCancel := context.WithTimeout(cntxt, contextTimeout)
		defer clusterMetaCreateCancel()
		if err := r.Create(clusterMetaCreateCtx, clusterMeta.DeepCopyObject(), &client.CreateOptions{}); err != nil {
			logger.Error(fmt.Errorf("{clusterMeta Create} %s", err), "")
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *ServerClusterReconciler) deleteServerCluster(cluster *k8sv1alpha1.ServerCluster, clusterMeta *k8sv1alpha1.ServerClusterMeta) (ctrl.Result, error) {
	if clusterMeta != nil && clusterMeta.Spec.ID > 0 && !clusterMeta.Spec.Reclaim {
		reply, err := r.Cred.ServerCluster().Delete(clusterMeta.Spec.ID)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("{deleteServerCluster} %s", err)
		}
		resp, err := http.ParseAPIResponse(reply.Data)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !resp.IsSuccess && resp.Meta.StatusCode != 404 {
			return ctrl.Result{}, fmt.Errorf("{deleteServerCluster} %s", fmt.Errorf(resp.Message))
		}
	}
	return r.deleteCRs(cluster, clusterMeta)
}

func (r *ServerClusterReconciler) deleteCRs(cluster *k8sv1alpha1.ServerCluster, clusterMeta *k8sv1alpha1.ServerClusterMeta) (ctrl.Result, error) {
	if clusterMeta != nil {
		_, err := r.deleteServerClusterMetaCR(clusterMeta)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("{deleteCRs} %s", err)
		}
	}

	return r.deleteServerClusterCR(cluster)
}

func (r *ServerClusterReconciler) deleteServerClusterCR(cluster *k8sv1alpha1.ServerCluster) (ctrl.Result, error) {
	cluster.ObjectMeta.SetFinalizers(nil)
	cluster.SetFinalizers(nil)
	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	if err := r.Update(ctx, cluster.DeepCopyObject(), &client.UpdateOptions{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("{deleteServerClusterCR} %s", err)
	}

	return ctrl.Result{}, nil
}

func (r *ServerClusterReconciler) deleteServerClusterMetaCR(clusterMeta *k8sv1alpha1.ServerClusterMeta) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	if err := r.Delete(ctx, clusterMeta.DeepCopyObject(), &client.DeleteOptions{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("{deleteServerClusterMetaCR} %s", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServerClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.ServerCluster{}).
		Complete(r)
}
