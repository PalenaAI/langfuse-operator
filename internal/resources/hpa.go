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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// BuildWebHPA constructs the desired HorizontalPodAutoscaler for the Langfuse Web component.
func BuildWebHPA(instance *v1alpha1.LangfuseInstance) *autoscalingv2.HorizontalPodAutoscaler {
	return buildHPA(
		WebName(instance),
		instance.Namespace,
		CommonLabels(instance, "web"),
		instance.Spec.Web.Autoscaling,
	)
}

// BuildWorkerHPA constructs the desired HorizontalPodAutoscaler for the Langfuse Worker component.
func BuildWorkerHPA(instance *v1alpha1.LangfuseInstance) *autoscalingv2.HorizontalPodAutoscaler {
	return buildHPA(
		WorkerName(instance),
		instance.Namespace,
		CommonLabels(instance, "worker"),
		instance.Spec.Worker.Autoscaling,
	)
}

func buildHPA(name, namespace string, labels map[string]string, spec *v1alpha1.AutoscalingSpec) *autoscalingv2.HorizontalPodAutoscaler {
	minReplicas := int32(1)
	if spec != nil && spec.MinReplicas != nil {
		minReplicas = *spec.MinReplicas
	}

	maxReplicas := int32(10)
	if spec != nil && spec.MaxReplicas > 0 {
		maxReplicas = spec.MaxReplicas
	}

	targetCPU := int32(80)
	if spec != nil && spec.TargetCPUUtilization != nil {
		targetCPU = *spec.TargetCPUUtilization
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &targetCPU,
						},
					},
				},
			},
		},
	}
}
