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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
	"github.com/PalenaAI/langfuse-operator/internal/langfuse"
)

// BuildWorkerDeployment constructs the desired Deployment for the Langfuse Worker component.
func BuildWorkerDeployment(instance *v1alpha1.LangfuseInstance, config *langfuse.Config) *appsv1.Deployment {
	labels := CommonLabels(instance, "worker")
	selectorLabels := SelectorLabels(instance, "worker")

	replicas := int32(1)
	if instance.Spec.Worker.Replicas != nil {
		replicas = *instance.Spec.Worker.Replicas
	}

	envVars := mergeEnv(config.CommonEnv, config.WorkerEnv, instance.Spec.Worker.ExtraEnv)

	container := corev1.Container{
		Name:      "langfuse-worker",
		Image:     containerImage(instance),
		Env:       envVars,
		Resources: resourceRequirements(instance.Spec.Worker.Resources),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"node", "-e", "process.exit(0)"},
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       30,
			FailureThreshold:    3,
		},
	}

	if instance.Spec.Security != nil {
		container.SecurityContext = containerSecurityContext(instance)
	}

	podSpec := corev1.PodSpec{
		Containers:   []corev1.Container{container},
		NodeSelector: instance.Spec.Worker.NodeSelector,
		Tolerations:  instance.Spec.Worker.Tolerations,
		Affinity:     instance.Spec.Worker.Affinity,
	}

	if len(instance.Spec.Image.PullSecrets) > 0 {
		podSpec.ImagePullSecrets = instance.Spec.Image.PullSecrets
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkerName(instance),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}

	return deployment
}
