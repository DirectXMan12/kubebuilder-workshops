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
	"strings"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webappv1 "github.com/directxman12/kubebuilder-workshops/api/v1"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	Scheme *runtime.Scheme
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=webapp.metamagical.io,resources=redis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.metamagical.io,resources=redis/status,verbs=get;update;patch

func (r *RedisReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("redis", req.NamespacedName)

	var redis webappv1.Redis
	if err := r.Get(ctx, req.NamespacedName, &redis); err != nil {
		if ignoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch redis")
		return ctrl.Result{}, err
	}

	if err := r.ensureLeader(ctx, log, &redis); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureFollowers(ctx, log, &redis); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Status().Update(ctx, &redis); err != nil {
		log.Error(err, "unable to update redis status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

type deplCfg struct {
	env      []core.EnvVar
	replicas *int32
	image    string
	role     string
}

func (r *RedisReconciler) ensureDeplAndSvc(ctx context.Context, log logr.Logger, redis *webappv1.Redis, cfg deplCfg) (*core.Service, error) {
	sel := map[string]string{
		"role":  cfg.role,
		"redis": redis.Name,
	}

	depl := &apps.Deployment{ObjectMeta: metav1.ObjectMeta{Name: redis.Name + strings.ToLower(cfg.role), Namespace: redis.Namespace}}
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, depl, func() error {
		if cfg.replicas != nil {
			replicas := *cfg.replicas
			depl.Spec.Replicas = &replicas
		} else {
			depl.Spec.Replicas = nil
		}

		depl.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: sel,
		}
		depl.Spec.Template.ObjectMeta.Labels = sel
		depl.Spec.Template.Spec = core.PodSpec{
			Containers: []core.Container{
				{
					Name:  "redis",
					Image: cfg.image,
					Ports: []core.ContainerPort{{ContainerPort: 6379}},
				},
			},
		}

		for _, envVar := range cfg.env {
			setEnv(&depl.Spec.Template.Spec.Containers[0], envVar.Name, envVar.Value)
		}

		if err := ctrl.SetControllerReference(redis, depl, r.Scheme); err != nil {
			return err
		}

		setCondition(&redis.Status.Conditions, webappv1.StatusCondition{
			Type:    "DeploymentUpToDate",
			Status:  webappv1.ConditionStatusHealthy,
			Reason:  "EnsuredDeployment",
			Message: "Ensured deployment was up to date",
		})

		return nil
	}); err != nil {
		log.Error(err, "unable to ensure deployment is up to date", "role", cfg.role)
		setCondition(&redis.Status.Conditions, webappv1.StatusCondition{
			Type:    cfg.role + "DeploymentUpToDate",
			Status:  webappv1.ConditionStatusUnhealthy,
			Reason:  "UpdateError",
			Message: "Unable to fetch or update deployment",
		})
		if err := r.Status().Update(ctx, redis); err != nil {
			log.Error(err, "unable to update redis status")
		}
		return nil, err
	}

	svcName := redis.Name + "-" + strings.ToLower(cfg.role)
	svc := &core.Service{ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: redis.Namespace}}
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if ownerRef := ownedByOther(svc, webappv1.GroupVersion, "Redis", redis.Name); ownerRef != nil {
			log.Info("cowardly refusing to take ownership of somebody else's service", "owner", ownerRef, "service", svc.Name)
			setCondition(&redis.Status.Conditions, webappv1.StatusCondition{
				Type:    cfg.role + "ServiceUpToDate",
				Status:  webappv1.ConditionStatusUnhealthy,
				Reason:  "OrphanService",
				Message: "Refusing to take ownership of somebody else's service",
			})
			return nil
		}

		svc.Spec.Selector = sel
		// stuff gets defaulted, so make sure to find the right port and modify
		// (otherwise we'll requeue forever)
		if len(svc.Spec.Ports) == 0 {
			svc.Spec.Ports = append(svc.Spec.Ports, core.ServicePort{})
		}
		svcPort := &svc.Spec.Ports[0]
		svcPort.Port = 6379
		svcPort.TargetPort = intstr.FromInt(6379)

		// set the owner so that garbage collection kicks in
		if err := ctrl.SetControllerReference(redis, svc, r.Scheme); err != nil {
			return err
		}
		setCondition(&redis.Status.Conditions, webappv1.StatusCondition{
			Type:    cfg.role + "ServiceUpToDate",
			Status:  webappv1.ConditionStatusHealthy,
			Reason:  "EnsuredService",
			Message: "Ensured service was up to date",
		})
		return nil
	}); err != nil {
		log.Error(err, "unable to ensure service is up to date", "role", cfg.role)
		setCondition(&redis.Status.Conditions, webappv1.StatusCondition{
			Type:    cfg.role + "ServiceUpToDate",
			Status:  webappv1.ConditionStatusUnhealthy,
			Reason:  "UpdateError",
			Message: "Unable to fetch or update deployment",
		})
		if err := r.Status().Update(ctx, redis); err != nil {
			log.Error(err, "unable to update redis status")
		}
		return svc, err
	}

	return svc, nil
}

func (r *RedisReconciler) ensureLeader(ctx context.Context, log logr.Logger, redis *webappv1.Redis) error {

	leaderSvc, err := r.ensureDeplAndSvc(ctx, log, redis, deplCfg{
		role:  "Leader",
		image: "k8s.gcr.io/redis:e2e",
	})
	if err != nil {
		return err
	}
	redis.Status.LeaderService = leaderSvc.Name

	return nil
}

func (r *RedisReconciler) ensureFollowers(ctx context.Context, log logr.Logger, redis *webappv1.Redis) error {
	followerSvc, err := r.ensureDeplAndSvc(ctx, log, redis, deplCfg{
		role:     "Follower",
		image:    "gcr.io/google_samples/gb-redisslave:v1",
		replicas: redis.Spec.FollowerReplicas,
		env: []core.EnvVar{
			{Name: "GET_HOSTS_FROM", Value: "env"},
			{Name: "REDIS_MASTER_SERVICE_HOST", Value: redis.Name + "-leader"},
		},
	})
	if err != nil {
		return err
	}
	redis.Status.FollowerService = followerSvc.Name

	return nil
}

func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.Redis{}).
		Owns(&apps.Deployment{}).
		Owns(&core.Service{}).
		Complete(r)
}
