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
	"github.com/PalenaAI/langfuse-operator/internal/langfuse"
)

const (
	clickHouseImage       = "clickhouse/clickhouse-server:24-alpine"
	clickHouseHTTPPort    = int32(8123)
	clickHouseNativePort  = int32(9000)
	clickHouseDataMount   = "/var/lib/clickhouse"
	clickHouseConfigMount = "/etc/clickhouse-server/config.d"

	defaultClickHouseStorageSize = "10Gi"
)

// ClickHouseName returns the name for ClickHouse resources.
func ClickHouseName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-clickhouse"
}

// BuildClickHouseStatefulSet constructs the desired StatefulSet for a single-node managed ClickHouse.
func BuildClickHouseStatefulSet(instance *v1alpha1.LangfuseInstance) *appsv1.StatefulSet {
	labels := CommonLabels(instance, "clickhouse")
	selectorLabels := SelectorLabels(instance, "clickhouse")
	name := ClickHouseName(instance)

	managed := instance.Spec.ClickHouse.Managed

	replicas := int32(1)
	if managed.Replicas != nil {
		replicas = *managed.Replicas
	}

	storageSize := defaultClickHouseStorageSize
	if managed.StorageSize != "" {
		storageSize = managed.StorageSize
	}

	env := clickHouseEnvVars(instance)

	container := corev1.Container{
		Name:  "clickhouse",
		Image: clickHouseImage,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: clickHouseHTTPPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "native",
				ContainerPort: clickHouseNativePort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env:       env,
		Resources: clickHouseResources(managed.Resources),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/ping",
					Port:   intstr.FromInt32(clickHouseHTTPPort),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			FailureThreshold:    3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/ping",
					Port:   intstr.FromInt32(clickHouseHTTPPort),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			FailureThreshold:    3,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: clickHouseDataMount,
			},
			{
				Name:      "config",
				MountPath: clickHouseConfigMount,
				ReadOnly:  true,
			},
		},
	}

	// Build PVC spec
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(storageSize),
			},
		},
	}
	if managed.StorageClass != "" {
		pvcSpec.StorageClassName = &managed.StorageClass
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: name,
									},
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: pvcSpec,
				},
			},
		},
	}
}

// BuildClickHouseService constructs the desired Service for ClickHouse.
func BuildClickHouseService(instance *v1alpha1.LangfuseInstance) *corev1.Service {
	labels := CommonLabels(instance, "clickhouse")
	selectorLabels := SelectorLabels(instance, "clickhouse")

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClickHouseName(instance),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       clickHouseHTTPPort,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "native",
					Port:       clickHouseNativePort,
					TargetPort: intstr.FromString("native"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// BuildClickHouseConfigMap constructs the desired ConfigMap with ClickHouse server configuration.
func BuildClickHouseConfigMap(instance *v1alpha1.LangfuseInstance) *corev1.ConfigMap {
	labels := CommonLabels(instance, "clickhouse")

	configXML := `<?xml version="1.0"?>
<clickhouse>
    <logger>
        <level>information</level>
        <console>1</console>
    </logger>
    <listen_host>0.0.0.0</listen_host>
    <http_port>8123</http_port>
    <tcp_port>9000</tcp_port>
    <max_connections>4096</max_connections>
    <keep_alive_timeout>3</keep_alive_timeout>
    <max_concurrent_queries>100</max_concurrent_queries>
    <mark_cache_size>5368709120</mark_cache_size>
</clickhouse>`

	usersXML := `<?xml version="1.0"?>
<clickhouse>
    <profiles>
        <default>
            <max_memory_usage>10000000000</max_memory_usage>
            <load_balancing>random</load_balancing>
        </default>
    </profiles>
    <quotas>
        <default>
            <interval>
                <duration>3600</duration>
                <queries>0</queries>
                <errors>0</errors>
                <result_rows>0</result_rows>
                <read_rows>0</read_rows>
                <execution_time>0</execution_time>
            </interval>
        </default>
    </quotas>
</clickhouse>`

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClickHouseName(instance),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"config.xml": configXML,
			"users.xml":  usersXML,
		},
	}
}

// clickHouseEnvVars returns environment variables for the ClickHouse container.
// If an explicit auth secret is provided, credentials are sourced from it;
// otherwise they come from the operator-generated secrets.
func clickHouseEnvVars(instance *v1alpha1.LangfuseInstance) []corev1.EnvVar {
	managed := instance.Spec.ClickHouse.Managed

	secretName := langfuse.GeneratedSecretName(instance)
	userKey := "clickhouse-username"
	passwordKey := "clickhouse-password"

	if managed.Auth != nil && managed.Auth.SecretRef != nil {
		secretName = managed.Auth.SecretRef.Name
		if k, ok := managed.Auth.SecretRef.Keys["username"]; ok {
			userKey = k
		}
		if k, ok := managed.Auth.SecretRef.Keys["password"]; ok {
			passwordKey = k
		}
	}

	return []corev1.EnvVar{
		{
			Name: "CLICKHOUSE_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  userKey,
				},
			},
		},
		{
			Name: "CLICKHOUSE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  passwordKey,
				},
			},
		},
	}
}

// clickHouseResources maps a resource preset (small, medium, large) to concrete
// resource requirements. If a custom spec is provided it takes precedence.
func clickHouseResources(spec *v1alpha1.ClickHouseResourceSpec) corev1.ResourceRequirements {
	if spec == nil {
		return clickHousePreset("small")
	}
	if spec.Preset == "custom" && spec.Custom != nil {
		return corev1.ResourceRequirements{
			Requests: spec.Custom.Requests,
			Limits:   spec.Custom.Limits,
		}
	}
	preset := spec.Preset
	if preset == "" {
		preset = "small"
	}
	return clickHousePreset(preset)
}

func clickHousePreset(preset string) corev1.ResourceRequirements {
	switch preset {
	case "medium":
		return corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		}
	case "large":
		return corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
			},
		}
	default: // "small" or unrecognized
		return corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		}
	}
}
