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

// ServerClusterTemplateReconciler reconciles a ServerClusterTemplate object
type ServerClusterTemplateReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Cred     *api.Clientset
	NStorage *netrisstorage.Storage
}

//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclustertemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclustertemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclustertemplates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ServerClusterTemplateReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("name", req.NamespacedName)
	debugLogger := logger.V(int(zapcore.WarnLevel))
	template := &k8sv1alpha1.ServerClusterTemplate{}

	u := uniReconciler{
		Client:      r.Client,
		Logger:      logger,
		DebugLogger: debugLogger,
		Cred:        r.Cred,
		NStorage:    r.NStorage,
	}

	templateCtx, templateCancel := context.WithTimeout(cntxt, contextTimeout)
	defer templateCancel()
	if err := r.Get(templateCtx, req.NamespacedName, template); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	templateMetaNamespaced := req.NamespacedName
	templateMetaNamespaced.Name = string(template.GetUID())
	templateMeta := &k8sv1alpha1.ServerClusterTemplateMeta{}
	metaFound := true

	templateMetaCtx, templateMetaCancel := context.WithTimeout(cntxt, contextTimeout)
	defer templateMetaCancel()
	if err := r.Get(templateMetaCtx, templateMetaNamespaced, templateMeta); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			metaFound = false
			templateMeta = nil
		} else {
			return ctrl.Result{}, err
		}
	}

	if template.DeletionTimestamp != nil {
		logger.Info("Go to delete")
		_, err := r.deleteServerClusterTemplate(template, templateMeta)
		if err != nil {
			logger.Error(fmt.Errorf("{deleteServerClusterTemplate} %s", err), "")
			return u.patchServerClusterTemplateStatus(template, "Failure", err.Error())
		}
		logger.Info("ServerClusterTemplate deleted")
		return ctrl.Result{}, nil
	}

	if serverClusterTemplateMustUpdateAnnotations(template) {
		debugLogger.Info("Setting default annotations")
		serverClusterTemplateUpdateDefaultAnnotations(template)
		templatePatchCtx, templatePatchCancel := context.WithTimeout(cntxt, contextTimeout)
		defer templatePatchCancel()
		err := r.Patch(templatePatchCtx, template.DeepCopyObject(), client.Merge, &client.PatchOptions{})
		if err != nil {
			logger.Error(fmt.Errorf("{Patch ServerClusterTemplate default annotations} %s", err), "")
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
		return ctrl.Result{}, nil
	}

	if metaFound {
		debugLogger.Info("Meta found")
		if serverClusterTemplateCompareFieldsForNewMeta(template, templateMeta) {
			debugLogger.Info("Generating New Meta")
			templateID := templateMeta.Spec.ID
			newTemplateMeta, err := r.ServerClusterTemplateToMeta(template)
			if err != nil {
				logger.Error(fmt.Errorf("{ServerClusterTemplateToMeta} %s", err), "")
				return u.patchServerClusterTemplateStatus(template, "Failure", err.Error())
			}
			templateMeta.Spec = newTemplateMeta.DeepCopy().Spec
			templateMeta.Spec.ID = templateID
			templateMeta.Spec.ServerClusterTemplateCRGeneration = template.GetGeneration()

			templateMetaUpdateCtx, templateMetaUpdateCancel := context.WithTimeout(cntxt, contextTimeout)
			defer templateMetaUpdateCancel()
			err = r.Update(templateMetaUpdateCtx, templateMeta.DeepCopyObject(), &client.UpdateOptions{})
			if err != nil {
				logger.Error(fmt.Errorf("{templateMeta Update} %s", err), "")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
		}
	} else {
		debugLogger.Info("Meta not found")
		if template.GetFinalizers() == nil {
			template.SetFinalizers([]string{"resource.k8s.netris.ai/delete"})

			templatePatchCtx, templatePatchCancel := context.WithTimeout(cntxt, contextTimeout)
			defer templatePatchCancel()
			err := r.Patch(templatePatchCtx, template.DeepCopyObject(), client.Merge, &client.PatchOptions{})
			if err != nil {
				logger.Error(fmt.Errorf("{Patch ServerClusterTemplate Finalizer} %s", err), "")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
			return ctrl.Result{}, nil
		}

		templateMeta, err := r.ServerClusterTemplateToMeta(template)
		if err != nil {
			logger.Error(fmt.Errorf("{ServerClusterTemplateToMeta} %s", err), "")
			return u.patchServerClusterTemplateStatus(template, "Failure", err.Error())
		}

		templateMeta.Spec.ServerClusterTemplateCRGeneration = template.GetGeneration()

		templateMetaCreateCtx, templateMetaCreateCancel := context.WithTimeout(cntxt, contextTimeout)
		defer templateMetaCreateCancel()
		if err := r.Create(templateMetaCreateCtx, templateMeta.DeepCopyObject(), &client.CreateOptions{}); err != nil {
			logger.Error(fmt.Errorf("{templateMeta Create} %s", err), "")
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *ServerClusterTemplateReconciler) deleteServerClusterTemplate(template *k8sv1alpha1.ServerClusterTemplate, templateMeta *k8sv1alpha1.ServerClusterTemplateMeta) (ctrl.Result, error) {
	if templateMeta != nil && templateMeta.Spec.ID > 0 && !templateMeta.Spec.Reclaim {
		reply, err := r.Cred.ServerClusterTemplate().Delete(templateMeta.Spec.ID)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("{deleteServerClusterTemplate} %s", err)
		}
		resp, err := http.ParseAPIResponse(reply.Data)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !resp.IsSuccess && resp.Meta.StatusCode != 404 {
			return ctrl.Result{}, fmt.Errorf("{deleteServerClusterTemplate} %s", fmt.Errorf(resp.Message))
		}
	}
	return r.deleteCRs(template, templateMeta)
}

func (r *ServerClusterTemplateReconciler) deleteCRs(template *k8sv1alpha1.ServerClusterTemplate, templateMeta *k8sv1alpha1.ServerClusterTemplateMeta) (ctrl.Result, error) {
	if templateMeta != nil {
		_, err := r.deleteServerClusterTemplateMetaCR(templateMeta)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("{deleteCRs} %s", err)
		}
	}

	return r.deleteServerClusterTemplateCR(template)
}

func (r *ServerClusterTemplateReconciler) deleteServerClusterTemplateCR(template *k8sv1alpha1.ServerClusterTemplate) (ctrl.Result, error) {
	template.ObjectMeta.SetFinalizers(nil)
	template.SetFinalizers(nil)
	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	if err := r.Update(ctx, template.DeepCopyObject(), &client.UpdateOptions{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("{deleteServerClusterTemplateCR} %s", err)
	}

	return ctrl.Result{}, nil
}

func (r *ServerClusterTemplateReconciler) deleteServerClusterTemplateMetaCR(templateMeta *k8sv1alpha1.ServerClusterTemplateMeta) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	if err := r.Delete(ctx, templateMeta.DeepCopyObject(), &client.DeleteOptions{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("{deleteServerClusterTemplateMetaCR} %s", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServerClusterTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.ServerClusterTemplate{}).
		Complete(r)
}
