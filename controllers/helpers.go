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
	"fmt"
	"net"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	webappv1 "workshop-code/api/v1"
)

func (r *GuestBookReconciler) desiredDeployment(book webappv1.GuestBook, redis webappv1.Redis) (appsv1.Deployment, error) {
	depl := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      book.Name,
			Namespace: book.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: book.Spec.Frontend.Replicas, // won't be nil because defaulting
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"guestbook": book.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"guestbook": book.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "frontend",
							Image: "gcr.io/google-samples/gb-frontend:v4",
							// NB(directxman12): sorry about these environment
							// variable names -- they're what the official
							// sample uses (since they used to be the official
							// terms for Redis).  We should change them now that Redis
							// has changed as well.
							Env: []corev1.EnvVar{
								{Name: "GET_HOSTS_FROM", Value: "env"},
								{Name: "REDIS_MASTER_SERVICE_HOST", Value: redis.Status.LeaderService},
								{Name: "REDIS_SLAVE_SERVICE_HOST", Value: redis.Status.FollowerService},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 80, Name: "http", Protocol: "TCP"},
							},
							Resources: *book.Spec.Frontend.Resources.DeepCopy(),
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(&book, &depl, r.Scheme); err != nil {
		return depl, err
	}

	return depl, nil
}

func (r *GuestBookReconciler) desiredService(book webappv1.GuestBook) (corev1.Service, error) {
	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      book.Name,
			Namespace: book.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 8080, Protocol: "TCP", TargetPort: intstr.FromString("http")},
			},
			Selector: map[string]string{"guestbook": book.Name},
			Type:     corev1.ServiceTypeLoadBalancer,
		},
	}

	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(&book, &svc, r.Scheme); err != nil {
		return svc, err
	}

	return svc, nil
}

func urlForService(svc corev1.Service, port int32) string {
	// notice that we unset this if it's not present -- we always want the
	// state to reflect what we observe.
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return ""
	}

	host := svc.Status.LoadBalancer.Ingress[0].Hostname
	if host == "" {
		host = svc.Status.LoadBalancer.Ingress[0].IP
	}

	return fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprintf("%v", port)))
}

func (r *GuestBookReconciler) booksUsingRedis(obj handler.MapObject) []ctrl.Request {
	listOptions := []client.ListOption{
		// matching our index
		client.MatchingField(".spec.redisName", obj.Meta.GetName()),
		// in the right namespace
		client.InNamespace(obj.Meta.GetNamespace()),
	}
	var list webappv1.GuestBookList
	if err := r.List(context.Background(), &list, listOptions...); err != nil {
		// TODO: we should log here!
		return nil
	}
	res := make([]ctrl.Request, len(list.Items))
	for i, book := range list.Items {
		res[i].Name = book.Name
		res[i].Namespace = book.Namespace
	}
	return res
}

func (r *RedisReconciler) leaderDeployment(redis webappv1.Redis) (appsv1.Deployment, error) {
	defOne := int32(1)
	depl := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      redis.Name + "-leader",
			Namespace: redis.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &defOne,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"redis": redis.Name, "role": "leader"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"redis": redis.Name, "role": "leader"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "leader",
							Image: "k8s.gcr.io/redis:e2e",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 6379, Name: "redis", Protocol: "TCP"},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewMilliQuantity(100000, resource.BinarySI),
								},
							},
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(&redis, &depl, r.Scheme); err != nil {
		return depl, err
	}

	return depl, nil
}

func (r *RedisReconciler) followerDeployment(redis webappv1.Redis) (appsv1.Deployment, error) {
	depl := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      redis.Name + "-follower",
			Namespace: redis.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: redis.Spec.FollowerReplicas, // won't be nil because defaulting
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"redis": redis.Name, "role": "follower"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"redis": redis.Name, "role": "follower"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "follower",
							Image: "gcr.io/google_samples/gb-redisslave:v3",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 6379, Name: "redis", Protocol: "TCP"},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewMilliQuantity(100000, resource.BinarySI),
								},
							},

							// NB(directxman12): sorry about these environment
							// variable names -- they're what the official
							// sample uses (since they used to be the official
							// terms for Redis).  We should change them now that Redis
							// has changed as well.
							Env: []corev1.EnvVar{
								{Name: "GET_HOSTS_FROM", Value: "env"},
								{Name: "REDIS_MASTER_SERVICE_HOST", Value: redis.Name + "-leader"},
								{Name: "REDIS_SLAVE_SERVICE_HOST", Value: redis.Name + "-follower"},
							},
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(&redis, &depl, r.Scheme); err != nil {
		return depl, err
	}

	return depl, nil
}

func (r *RedisReconciler) desiredService(redis webappv1.Redis, role string) (corev1.Service, error) {
	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      redis.Name + "-" + role,
			Namespace: redis.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "redis", Port: 6379, Protocol: "TCP", TargetPort: intstr.FromString("redis")},
			},
			Selector: map[string]string{"redis": redis.Name, "role": role},
		},
	}

	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(&redis, &svc, r.Scheme); err != nil {
		return svc, err
	}

	return svc, nil
}
