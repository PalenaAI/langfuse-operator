/*
Copyright 2026.

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

package resources

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

const (
	redisImage         = "redis:7-alpine"
	redisPort    int32 = 6379
	redisDataDir       = "/data"
)

// RedisName returns the name for Redis resources.
func RedisName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-redis"
}

// RedisServiceName returns the name for the Redis Service.
func RedisServiceName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-redis"
}

// BuildRedisStatefulSet constructs the desired StatefulSet for a single-node managed Redis instance.
func BuildRedisStatefulSet(instance *v1alpha1.LangfuseInstance) *appsv1.StatefulSet {
	labels := CommonLabels(instance, "redis")
	selectorLabels := SelectorLabels(instance, "redis")

	replicas := int32(1)
	secretName := instance.Name + "-generated-secrets"

	storageSize := "1Gi"
	if instance.Spec.Redis != nil && instance.Spec.Redis.Managed != nil && instance.Spec.Redis.Managed.StorageSize != "" {
		storageSize = instance.Spec.Redis.Managed.StorageSize
	}

	container := corev1.Container{
		Name:  "redis",
		Image: redisImage,
		Command: []string{
			"redis-server",
			"--requirepass", "$(REDIS_PASSWORD)",
			"--appendonly", "yes",
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "redis",
				ContainerPort: redisPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name: "REDIS_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: "redis-password",
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: redisDataDir,
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"redis-cli", "-a", "$(REDIS_PASSWORD)", "ping"},
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       10,
			FailureThreshold:    3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"redis-cli", "-a", "$(REDIS_PASSWORD)", "ping"},
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			FailureThreshold:    3,
		},
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RedisName(instance),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: RedisServiceName(instance),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(storageSize),
							},
						},
					},
				},
			},
		},
	}
}

// BuildRedisService constructs the desired ClusterIP Service for the managed Redis instance.
func BuildRedisService(instance *v1alpha1.LangfuseInstance) *corev1.Service {
	labels := CommonLabels(instance, "redis")
	selectorLabels := SelectorLabels(instance, "redis")

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RedisServiceName(instance),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "redis",
					Port:       redisPort,
					TargetPort: intstr.FromString("redis"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
