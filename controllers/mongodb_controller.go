/*
Copyright 2019 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbs "github.com/pwittrock/kubebuilder-workshop/api/v1alpha1"
	"github.com/pwittrock/kubebuilder-workshop/util"
)

// MongoDBReconciler reconciles a MongoDB object
type MongoDBReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=databases.example.com,resources=mongodbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=databases.example.com,resources=mongodbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

func (r *MongoDBReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("mongodb", req.NamespacedName)

	// Fetch the MongoDB instance
	mongo := &dbs.MongoDB{}
	if err := r.Get(ctx, req.NamespacedName, mongo); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Generate the Service object managed by MongoDB
	svc := &corev1.Service{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      req.Name + "-mongodb-service",
			Namespace: req.Namespace,
		},
	}
	// either create or update the service as appropriate
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		// set the service fields to their appropriate values
		util.SetServiceFields(svc, mongo)

		// add the owner reference marking this as owned by the MongoDB object so the service
		// gets garbage-collected when the MongoDB object is deleted.
		return ctrl.SetControllerReference(mongo, svc, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Generate the StatefulSet object managed by MongoDB
	stateful := &appsv1.StatefulSet{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      req.Name + "-mongodb-statefulset",
			Namespace: req.Namespace,
		},
	}
	_, err = ctrl.CreateOrUpdate(ctx, r.Client, stateful, func() error {
		// set the statefulset fields to their appropriate values
		util.SetStatefulSetFields(stateful, svc, mongo, mongo.Spec.Replicas, mongo.Spec.Storage)

		// add the owner reference marking this as owned by the MongoDB object so the service
		// gets garbage-collected when the MongoDB object is deleted.
		return ctrl.SetControllerReference(mongo, stateful, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update the MongoDB status so it gets published to users
	mongo.Status.StatefulSetStatus = stateful.Status
	mongo.Status.ServiceStatus = svc.Status
	mongo.Status.ClusterIP = svc.Spec.ClusterIP

	if err := r.Status().Update(ctx, mongo); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("updated mongo intance")

	return ctrl.Result{}, nil
}

func (r *MongoDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbs.MongoDB{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
