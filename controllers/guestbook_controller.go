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
	"fmt"
	"net"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	webappv1 "github.com/directxman12/kubebuilder-workshops/api/v1"
)

func ignoreNotFound(err error) error {
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func ownedByOther(obj metav1.Object, apiVersion schema.GroupVersion, kind, name string) *metav1.OwnerReference {
	if ownerRef := metav1.GetControllerOf(obj); ownerRef != nil && (ownerRef.Name != name || ownerRef.Kind != kind || ownerRef.APIVersion != apiVersion.String()) {
		return ownerRef
	}
	return nil
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
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;update;watch;list

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

	redisFollowerSvc := ""
	redisLeaderSvc := ""
	if app.Spec.RedisName != "" {
		var redis webappv1.Redis
		if err := r.Get(ctx, types.NamespacedName{Name: app.Spec.RedisName, Namespace: app.Namespace}, &redis); err != nil {
			log.Error(err, "unable to fetch corresponding redis", "redis", app.Spec.RedisName)
			return ctrl.Result{}, ignoreNotFound(err)
		}
		redisFollowerSvc = redis.Status.FollowerService
		redisLeaderSvc = redis.Status.LeaderService
		log.Info("using redis object", "redis", redis, "follower service", redisFollowerSvc, "leader service", redisLeaderSvc)
	}

	var conditions []webappv1.StatusCondition
	defer func() {
		app.Status.Conditions = conditions
		if err := r.Status().Patch(ctx, &app, client.Apply, client.ForceOwnership, client.FieldOwner("guestbook-controller")); err != nil {
			log.Error(err, "unable to app update guestbook status")
		}
	}()

	// create or update the deployment
	depl := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			// we'll make things simple by matching name to the name of our guestbook
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: apps.DeploymentSpec{
			Replicas: app.Spec.Frontend.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"guestbook": req.Name},
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"guestbook": req.Name}},
				Spec: core.PodSpec{
					Containers: []core.Container{
						{
							Name: "frontend",
							Image: "gcr.io/google-samples/gb-frontend:v4",
							Env: []core.EnvVar{
								{Name: "GET_HOSTS_FROM", Value: "env"},
								{Name: "REDIS_SLAVE_SERVICE_HOST", Value: redisFollowerSvc},
								{Name: "REDIS_MASTER_SERVICE_HOST", Value: redisLeaderSvc},
							},
							Resources: app.Spec.Frontend.Resources,
							Ports: []core.ContainerPort{
								{Name: "http", ContainerPort: 80},
							},
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(&app, depl, r.Scheme); err != nil {
		// TODO: preserve conditions?
		return ctrl.Result{}, err
	}

	if err := r.Patch(ctx, depl, client.Apply, client.ForceOwnership, client.FieldOwner("guestbook-controller")); err != nil {
		conditions = append(conditions, webappv1.StatusCondition{
			Type:    "DeploymentUpToDate",
			Status:  webappv1.ConditionStatusUnhealthy,
			Reason:  "UpdateError",
			Message: "Unable to fetch or update deployment",
		})
		return ctrl.Result{}, err
	}

	conditions = append(conditions, webappv1.StatusCondition{
		Type:    "DeploymentUpToDate",
		Status:  webappv1.ConditionStatusHealthy,
		Reason:  "EnsuredDeployment",
		Message: "Ensured deployment was up to date",
	})


	// ensure there's a service, too!
	port := int32(80)
	if app.Spec.Frontend.ServingPort != 0 {
		port = app.Spec.Frontend.ServingPort
	}
	svc := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: core.ServiceSpec{
			Selector: map[string]string{"guestbook": req.Name},
			Ports: []core.ServicePort{
				{
					Name: "http",
					Port: port,
					TargetPort: intstr.FromString("http"),
				},
			},
			Type: "LoadBalancer",
		},
	}
	if err := ctrl.SetControllerReference(&app, svc, r.Scheme); err != nil {
		// TODO: preserve conditions
		return ctrl.Result{}, err
	}
	if err := r.Patch(ctx, depl, client.Apply, client.ForceOwnership, client.FieldOwner("guestbook-controller")); err != nil {
		conditions = append(conditions, webappv1.StatusCondition{
			Type:    "ServiceUpToDate",
			Status:  webappv1.ConditionStatusUnhealthy,
			Reason:  "UpdateError",
			Message: "Unable to fetch or update service",
		})
		return ctrl.Result{}, err
	}

	conditions = append(conditions, webappv1.StatusCondition{
		Type:    "ServiceUpToDate",
		Status:  webappv1.ConditionStatusHealthy,
		Reason:  "EnsuredService",
		Message: "Ensured service was up to date",
	})

	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		host := svc.Status.LoadBalancer.Ingress[0].Hostname
		if host == "" {
			host = svc.Status.LoadBalancer.Ingress[0].IP
		}
		app.Status.URL = fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprintf("%v", port)))
	} else {
		app.Status.URL = ""
	}

	// status is updated in defer

	return ctrl.Result{}, nil
}

func (r *GuestBookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mgr.GetFieldIndexer().IndexField(&webappv1.GuestBook{}, ".spec.redisName", func(obj runtime.Object) []string {
		redisName := obj.(*webappv1.GuestBook).Spec.RedisName
		if redisName == "" {
			return nil
		}
		return []string{redisName}
	})
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.GuestBook{}).
		Owns(&apps.Deployment{}).
		Owns(&core.Service{}).
		Watches(&source.Kind{Type: &webappv1.Redis{}}, &handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []ctrl.Request {
				var apps webappv1.GuestBookList
				if err := r.List(context.Background(), &apps, client.InNamespace(obj.Meta.GetNamespace()), client.MatchingField(".spec.redisName", obj.Meta.GetName())); err != nil {
					r.Log.Info("unable to get webapps for redis", "redis", obj)
					return nil
				}

				res := make([]ctrl.Request, len(apps.Items))
				for i, app := range apps.Items {
					res[i].Name = app.Name
					res[i].Namespace = app.Namespace
				}
				return res
			}),
		}).
		Complete(r)
}
