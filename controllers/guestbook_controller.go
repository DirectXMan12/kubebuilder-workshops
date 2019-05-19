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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webappv1 "github.com/directxman12/kubebuilder-workshops/api/v1"
)

// GuestBookReconciler reconciles a GuestBook object
type GuestBookReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=webapp.metamagical.io,resources=guestbooks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.metamagical.io,resources=guestbooks/status,verbs=get;update;patch

func (r *GuestBookReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("guestbook", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *GuestBookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.GuestBook{}).
		Complete(r)
}
