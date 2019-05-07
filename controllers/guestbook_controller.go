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
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webappv1 "github.com/directxman12/kubebuilder-workshops/api/v1"
)

func ignoreNotFound(err error) error {
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// GuestBookReconciler reconciles a GuestBook object
type GuestBookReconciler struct {
	Scheme *runtime.Scheme
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=webapp.metamagical.io,resources=guestbooks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.metamagical.io,resources=guestbooks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;update;watch;list

func (r *GuestBookReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("guestbook", req.NamespacedName)

	// first, fetch our guestbook
	var app webappv1.GuestBook
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		// it might be not found if this is a delete request
		if ignoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch guestbook")
		return ctrl.Result{}, err
	}

	// create or update the deployment
	depl := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			// we'll make things simple by matching name to the name of our guestbook
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, depl, func() error {
		if ownerRef := metav1.GetControllerOf(depl); ownerRef != nil && (ownerRef.Name != req.Name || ownerRef.Kind != "GuestBook" || ownerRef.APIVersion != webappv1.GroupVersion.String()) {
			log.Info("cowardly refusing to take ownership of somebody else's deployment", "owner", ownerRef)

			// TODO: conditions
			return nil
		}

		// NB: CreateOrUpdate (and all client methods) modify the passed-in object, so we don't
		// ever want a direct pointer to our webapp.

		// set the replicas
		replicas := int32(1)
		if app.Spec.Frontend.Replicas != nil {
			replicas = *app.Spec.Frontend.Replicas
		}
		depl.Spec.Replicas = &replicas

		// set a label for our service and deployment
		if depl.Spec.Template.ObjectMeta.Labels == nil {
			depl.Spec.Template.ObjectMeta.Labels = map[string]string{}
		}
		depl.Spec.Template.ObjectMeta.Labels["guestbook"] = req.Name
		depl.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{
			"guestbook": req.Name,
		}}

		// make sure we actually run what we want, though
		var cont *core.Container
		// find the right container
		containers := depl.Spec.Template.Spec.Containers
		for i, iterCont := range containers {
			if iterCont.Name == "frontend" {
				cont = &depl.Spec.Template.Spec.Containers[i]
				break
			}
		}
		if cont == nil {
			depl.Spec.Template.Spec.Containers = append(depl.Spec.Template.Spec.Containers, core.Container{
				Name: "frontend",
			})
			cont = &depl.Spec.Template.Spec.Containers[len(depl.Spec.Template.Spec.Containers)-1]
		}

		// (this gets easier with server-side apply)
		cont.Image = "gcr.io/google-samples/gb-frontend:v4"

		// and again for env
		if app.Spec.UseDNS {
			setEnv(cont, "GET_HOSTS_FROM", "dns")
		} else {
			setEnv(cont, "GET_HOSTS_FROM", "env")
			setEnv(cont, "REDIS_SLAVE_SERVICE_HOST", req.Name+"-redis")
		}

		// copy resources
		if cont.Resources.Requests == nil {
			cont.Resources.Requests = make(core.ResourceList)
		}
		for res, val := range app.Spec.Frontend.Resources.Requests {
			cont.Resources.Requests[res] = val
		}
		if cont.Resources.Limits == nil {
			cont.Resources.Limits = make(core.ResourceList)
		}
		for res, val := range app.Spec.Frontend.Resources.Limits {
			cont.Resources.Limits[res] = val
		}

		// and again for the port
		var port *core.ContainerPort
		for i, iterPort := range cont.Ports {
			if iterPort.Name == "http" {
				port = &cont.Ports[i]
				break
			}
		}
		if port == nil {
			cont.Ports = append(cont.Ports, core.ContainerPort{Name: "http"})
			port = &cont.Ports[len(cont.Ports)-1]
		}
		port.ContainerPort = 80

		// set the owner so that garbage collection kicks in
		if err := ctrl.SetControllerReference(&app, depl, r.Scheme); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Error(err, "unable to ensure deployment is correct")
		return ctrl.Result{}, err
	}

	// ensure there's a service, too!
	svc := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if ownerRef := metav1.GetControllerOf(svc); ownerRef != nil && (ownerRef.Name != req.Name || ownerRef.Kind != "GuestBook" || ownerRef.APIVersion != webappv1.GroupVersion.String()) {
			log.Info("cowardly refusing to take ownership of somebody else's service", "owner", ownerRef)
			// TODO: conditions
			return nil
		}

		port := int32(80)
		if app.Spec.Frontend.ServingPort != 0 {
			port = app.Spec.Frontend.ServingPort
		}
		svc.Spec.Selector = map[string]string{"guestbook": req.Name}
		// stuff gets defaulted, so make sure to find the right port and modify
		// (otherwise we'll requeue forever)
		if len(svc.Spec.Ports) == 0 {
			svc.Spec.Ports = append(svc.Spec.Ports, core.ServicePort{})
		}
		svcPort := &svc.Spec.Ports[0]
		svcPort.Name = "http"
		svcPort.Port = port
		svcPort.TargetPort = intstr.FromString("http")

		svc.Spec.Type = "LoadBalancer"

		// set the owner so that garbage collection kicks in
		if err := ctrl.SetControllerReference(&app, svc, r.Scheme); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Error(err, "unable to ensure service is correct")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func setEnv(cont *core.Container, key, val string) {
	var envVar *core.EnvVar
	for i, iterVar := range cont.Env {
		if iterVar.Name == key {
			envVar = &cont.Env[i] // index to avoid capturing the iteration variable
			break
		}
	}
	if envVar == nil {
		cont.Env = append(cont.Env, core.EnvVar{
			Name: key,
		})
		envVar = &cont.Env[len(cont.Env)-1]
	}
	envVar.Value = val
}

func (r *GuestBookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.GuestBook{}).
		Owns(&apps.Deployment{}).
		Owns(&core.Service{}).
		Complete(r)
}
