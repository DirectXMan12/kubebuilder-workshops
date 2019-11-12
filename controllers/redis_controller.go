/*
Copyright 2019 Google LLC

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

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webappv1 "workshop-code/api/v1"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=webapp.metamagical.dev,resources=redis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.metamagical.dev,resources=redis/status,verbs=get;update;patch

func (r *RedisReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("redis", req.NamespacedName)

	log.Info("reconciling redis")

	var redis webappv1.Redis
	if err := r.Get(ctx, req.NamespacedName, &redis); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	leaderDepl, err := r.leaderDeployment(redis)
	if err != nil {
		return ctrl.Result{}, err
	}
	leaderSvc, err := r.desiredService(redis, "leader")
	if err != nil {
		return ctrl.Result{}, err
	}

	followerDepl, err := r.followerDeployment(redis)
	if err != nil {
		return ctrl.Result{}, err
	}
	followerSvc, err := r.desiredService(redis, "follower")
	if err != nil {
		return ctrl.Result{}, err
	}

	applyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("redis-controller")}

	err = r.Patch(ctx, &leaderDepl, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.Patch(ctx, &leaderSvc, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.Patch(ctx, &followerDepl, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.Patch(ctx, &followerSvc, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	redis.Status.LeaderService = leaderSvc.Name
	redis.Status.FollowerService = followerSvc.Name

	if err := r.Status().Update(ctx, &redis); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("reconciled redis")
	return ctrl.Result{}, nil
}

func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.Redis{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
