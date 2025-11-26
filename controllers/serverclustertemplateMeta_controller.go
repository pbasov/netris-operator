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
	"github.com/netrisai/netriswebapi/v2/types/serverclustertemplate"
)

// ServerClusterTemplateMetaReconciler reconciles a ServerClusterTemplateMeta object
type ServerClusterTemplateMetaReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Cred     *api.Clientset
	NStorage *netrisstorage.Storage
}

//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclustertemplatesmeta,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclustertemplatesmeta/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.netris.ai,resources=serverclustertemplatesmeta/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ServerClusterTemplateMetaReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	debugLogger := r.Log.WithValues("name", req.NamespacedName).V(int(zapcore.WarnLevel))

	templateMeta := &k8sv1alpha1.ServerClusterTemplateMeta{}
	templateCR := &k8sv1alpha1.ServerClusterTemplate{}
	templateMetaCtx, templateMetaCancel := context.WithTimeout(cntxt, contextTimeout)
	defer templateMetaCancel()
	if err := r.Get(templateMetaCtx, req.NamespacedName, templateMeta); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger := r.Log.WithValues("name", fmt.Sprintf("%s/%s", req.NamespacedName.Namespace, templateMeta.Spec.ServerClusterTemplateName))
	debugLogger = logger.V(int(zapcore.WarnLevel))

	u := uniReconciler{
		Client:      r.Client,
		Logger:      logger,
		DebugLogger: debugLogger,
		Cred:        r.Cred,
		NStorage:    r.NStorage,
	}

	provisionState := "OK"

	templateNN := req.NamespacedName
	templateNN.Name = templateMeta.Spec.ServerClusterTemplateName
	templateNNCtx, templateNNCancel := context.WithTimeout(cntxt, contextTimeout)
	defer templateNNCancel()
	if err := r.Get(templateNNCtx, templateNN, templateCR); err != nil {
		if errors.IsNotFound(err) {
			debugLogger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if templateMeta.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	if templateMeta.Spec.ID == 0 {
		debugLogger.Info("ID Not found in meta")
		if templateMeta.Spec.Imported {
			logger.Info("Importing server cluster template")
			debugLogger.Info("Imported yaml mode. Finding ServerClusterTemplate by name")
			if apiTemplate, ok := r.NStorage.ServerClusterTemplateStorage.FindByName(templateMeta.Spec.ServerClusterTemplateName); ok {
				debugLogger.Info("Imported yaml mode. ServerClusterTemplate found")
				templateMeta.Spec.ID = apiTemplate.ID

				templateMetaPatchCtx, templateMetaPatchCancel := context.WithTimeout(cntxt, contextTimeout)
				defer templateMetaPatchCancel()
				err := r.Patch(templateMetaPatchCtx, templateMeta.DeepCopyObject(), client.Merge, &client.PatchOptions{})
				if err != nil {
					logger.Error(fmt.Errorf("{patch templateMeta.Spec.ID} %s", err), "")
					return u.patchServerClusterTemplateStatus(templateCR, "Failure", err.Error())
				}
				debugLogger.Info("Imported yaml mode. ID patched")
				logger.Info("ServerClusterTemplate imported")
				return ctrl.Result{RequeueAfter: requeueInterval}, nil
			}
			logger.Info("ServerClusterTemplate not found for import")
			debugLogger.Info("Imported yaml mode. ServerClusterTemplate not found")
		}

		logger.Info("Creating ServerClusterTemplate")
		if _, err, errMsg := r.createServerClusterTemplate(templateMeta); err != nil {
			logger.Error(fmt.Errorf("{createServerClusterTemplate} %s", err), "")
			return u.patchServerClusterTemplateStatus(templateCR, "Failure", errMsg.Error())
		}
		logger.Info("ServerClusterTemplate Created")
	} else {
		if apiTemplate, ok := r.NStorage.ServerClusterTemplateStorage.FindByID(templateMeta.Spec.ID); ok {
			debugLogger.Info("Comparing ServerClusterTemplateMeta with Netris ServerClusterTemplate")

			if ok := compareServerClusterTemplateMetaAPI(templateMeta, apiTemplate, u); ok {
				debugLogger.Info("Nothing Changed")
			} else {
				debugLogger.Info("Go to update ServerClusterTemplate in Netris")
				logger.Info("Updating ServerClusterTemplate")
				templateUpdate := ServerClusterTemplateMetaToNetris(templateMeta)

				js, _ := json.Marshal(templateUpdate)
				debugLogger.Info("templateUpdate", "payload", string(js))

				_, err, errMsg := updateServerClusterTemplate(templateMeta.Spec.ID, templateUpdate, r.Cred)
				if err != nil {
					logger.Error(fmt.Errorf("{updateServerClusterTemplate} %s", err), "")
					return u.patchServerClusterTemplateStatus(templateCR, "Failure", errMsg.Error())
				}
				logger.Info("ServerClusterTemplate Updated")
			}
		} else {
			debugLogger.Info("ServerClusterTemplate not found in Netris")
			debugLogger.Info("Going to create ServerClusterTemplate")
			logger.Info("Creating ServerClusterTemplate")
			if _, err, errMsg := r.createServerClusterTemplate(templateMeta); err != nil {
				logger.Error(fmt.Errorf("{createServerClusterTemplate} %s", err), "")
				return u.patchServerClusterTemplateStatus(templateCR, "Failure", errMsg.Error())
			}
			logger.Info("ServerClusterTemplate Created")
		}
	}

	return u.patchServerClusterTemplateStatus(templateCR, provisionState, "Success")
}

func (r *ServerClusterTemplateMetaReconciler) createServerClusterTemplate(templateMeta *k8sv1alpha1.ServerClusterTemplateMeta) (ctrl.Result, error, error) {
	debugLogger := r.Log.WithValues(
		"name", fmt.Sprintf("%s/%s", templateMeta.Namespace, templateMeta.Spec.ServerClusterTemplateName),
		"templateName", templateMeta.Spec.ServerClusterTemplateCRGeneration,
	).V(int(zapcore.WarnLevel))

	templateAdd := ServerClusterTemplateMetaToNetris(templateMeta)

	js, _ := json.Marshal(templateAdd)
	debugLogger.Info("templateToAdd", "payload", string(js))

	reply, err := r.Cred.ServerClusterTemplate().Add(templateAdd)
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

	debugLogger.Info("ServerClusterTemplate Created", "id", idStruct.ID)

	templateMeta.Spec.ID = idStruct.ID

	ctx, cancel := context.WithTimeout(cntxt, contextTimeout)
	defer cancel()
	err = r.Patch(ctx, templateMeta.DeepCopyObject(), client.Merge, &client.PatchOptions{})
	if err != nil {
		return ctrl.Result{}, err, err
	}

	debugLogger.Info("ID patched to meta", "id", idStruct.ID)
	return ctrl.Result{}, nil, nil
}

func updateServerClusterTemplate(id int, template *serverclustertemplate.ServerClusterTemplateW, cred *api.Clientset) (ctrl.Result, error, error) {
	reply, err := cred.ServerClusterTemplate().Update(id, template)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("{updateServerClusterTemplate} %s", err), err
	}
	resp, err := http.ParseAPIResponse(reply.Data)
	if err != nil {
		return ctrl.Result{}, err, err
	}
	if !resp.IsSuccess {
		return ctrl.Result{}, fmt.Errorf("{updateServerClusterTemplate} %s", fmt.Errorf(resp.Message)), fmt.Errorf(resp.Message)
	}

	return ctrl.Result{}, nil, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServerClusterTemplateMetaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.ServerClusterTemplateMeta{}).
		Complete(r)
}
