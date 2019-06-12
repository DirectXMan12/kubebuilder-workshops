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

package util

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// SetStatefulSetFields sets fields on a appsv1.StatefulSet pointer generated for the MongoDB instance
// object: MongoDB instance
// replicas: the number of replicas for the MongoDB instance
// storage: the size of the storage for the MongoDB instance (e.g. 100Gi)
func SetStatefulSetFields(ss *appsv1.StatefulSet, service *corev1.Service, mongo metav1.Object, replicas *int32, storage *string) {
	gracePeriodTerm := int64(10)

	if replicas == nil {
		r := int32(1)
		replicas = &r
	}
	if storage == nil {
		s := "100Gi"
		storage = &s
	}

	copyLabels := mongo.GetLabels()
	if copyLabels == nil {
		copyLabels = map[string]string{}
	}

	labels := map[string]string{}
	for k, v := range copyLabels {
		labels[k] = v
	}
	labels["mongodb-statefuleset"] = mongo.GetName()

	rl := corev1.ResourceList{}
	rl["storage"] = resource.MustParse(*storage)

	ss.Labels = labels
	ss.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{"mongodb-statefulset": mongo.GetName()},
	}
	ss.Spec.ServiceName = service.Name
	ss.Spec.Replicas = replicas
	ss.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: ss.Spec.Selector.MatchLabels,
		},

		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &gracePeriodTerm,
			Containers: []corev1.Container{
				{
					Name:         "mongo",
					Image:        "mongo",
					Command:      []string{"mongod", "--replSet", "rs0", "--smallfiles", "--noprealloc", "--bind_ip_all"},
					Ports:        []corev1.ContainerPort{{ContainerPort: 27017}},
					VolumeMounts: []corev1.VolumeMount{{Name: "mongo-persistent-storage", MountPath: "/data/db"}},
				},
				{
					Name:  "mongo-sidecar",
					Image: "cvallance/mongo-k8s-sidecar",
					Env:   []corev1.EnvVar{{Name: "MONGO_SIDECAR_POD_LABELS", Value: "role=mongo,environment=test"}},
				},
			},
		},
	}
	ss.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "mongo-persistent-storage"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
				Resources: corev1.ResourceRequirements{
					Requests: rl,
				},
			},
		},
	}
}

// SetServiceFields sets fields on the Service object
func SetServiceFields(service *corev1.Service, mongo metav1.Object) {
	copyLabels := mongo.GetLabels()
	if copyLabels == nil {
		copyLabels = map[string]string{}
	}
	labels := map[string]string{}
	for k, v := range copyLabels {
		labels[k] = v
	}
	service.Labels = labels

	service.Spec.Ports = []corev1.ServicePort{
		{Port: 27017, TargetPort: intstr.IntOrString{IntVal: 27017, Type: intstr.Int}},
	}
	service.Spec.Selector = map[string]string{"mongodb-statefulset": mongo.GetName()}
}
