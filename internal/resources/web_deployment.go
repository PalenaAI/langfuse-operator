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
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
	"github.com/PalenaAI/langfuse-operator/internal/langfuse"
)

// BuildWebDeployment constructs the desired Deployment for the Langfuse Web component.
func BuildWebDeployment(instance *v1alpha1.LangfuseInstance, config *langfuse.Config) *appsv1.Deployment {
	labels := CommonLabels(instance, "web")
	selectorLabels := SelectorLabels(instance, "web")

	replicas := int32(1)
	if instance.Spec.Web.Replicas != nil {
		replicas = *instance.Spec.Web.Replicas
	}

	envVars := mergeEnv(config.CommonEnv, config.WebEnv, instance.Spec.Web.ExtraEnv)

	container := corev1.Container{
		Name:  "langfuse-web",
		Image: containerImage(instance),
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: 3000,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env:            envVars,
		Resources:      resourceRequirements(instance.Spec.Web.Resources),
		LivenessProbe:  httpProbe("/api/public/health", 3000, 30, 10, 3),
		ReadinessProbe: httpProbe("/api/public/health", 3000, 5, 10, 3),
		// Datastore TLS mounts (config.VolumeMounts) come first so they share
		// the same fixed paths the env vars point at, regardless of any
		// user-supplied extra mounts.
		VolumeMounts: mergeVolumeMounts(config.VolumeMounts, instance.Spec.Web.ExtraVolumeMounts),
	}

	if instance.Spec.Security != nil {
		container.SecurityContext = containerSecurityContext(instance)
	}

	podSpec := corev1.PodSpec{
		Containers:   []corev1.Container{container},
		Volumes:      mergeVolumes(config.Volumes, instance.Spec.Web.ExtraVolumes),
		NodeSelector: instance.Spec.Web.NodeSelector,
		Tolerations:  instance.Spec.Web.Tolerations,
		Affinity:     instance.Spec.Web.Affinity,
	}

	if len(instance.Spec.Image.PullSecrets) > 0 {
		podSpec.ImagePullSecrets = instance.Spec.Image.PullSecrets
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebName(instance),
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

func containerImage(instance *v1alpha1.LangfuseInstance) string {
	repo := instance.Spec.Image.Repository
	if repo == "" {
		repo = "langfuse/langfuse"
	}
	return repo + ":" + instance.Spec.Image.Tag
}

// workerContainerImage resolves the image for the Worker component. Langfuse v3
// ships the BullMQ queue consumer as a separate image (langfuse/langfuse-worker);
// the main langfuse/langfuse image only serves the web app/API and never drains
// the ingestion queues. The repository defaults to langfuse/langfuse-worker and
// the tag defaults to spec.image.tag so Web and Worker stay on the same version.
// Both are overridable via spec.worker.image.
func workerContainerImage(instance *v1alpha1.LangfuseInstance) string {
	repo := "langfuse/langfuse-worker"
	tag := instance.Spec.Image.Tag
	if img := instance.Spec.Worker.Image; img != nil {
		if img.Repository != "" {
			repo = img.Repository
		}
		if img.Tag != "" {
			tag = img.Tag
		}
	}
	return repo + ":" + tag
}

func httpProbe(path string, port int, initialDelay, period, failureThreshold int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   path,
				Port:   intstr.FromInt32(int32(port)),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
		FailureThreshold:    failureThreshold,
	}
}

func containerSecurityContext(instance *v1alpha1.LangfuseInstance) *corev1.SecurityContext {
	sc := &corev1.SecurityContext{}
	if instance.Spec.Security.ReadOnlyRootFilesystem != nil {
		sc.ReadOnlyRootFilesystem = instance.Spec.Security.ReadOnlyRootFilesystem
	}
	if instance.Spec.Security.RunAsNonRoot != nil {
		sc.RunAsNonRoot = instance.Spec.Security.RunAsNonRoot
	}
	return sc
}

func resourceRequirements(r *v1alpha1.ResourceRequirements) corev1.ResourceRequirements {
	if r == nil {
		return corev1.ResourceRequirements{}
	}
	return corev1.ResourceRequirements{
		Requests: r.Requests,
		Limits:   r.Limits,
	}
}

func mergeEnv(envSets ...[]corev1.EnvVar) []corev1.EnvVar {
	var result []corev1.EnvVar
	for _, envs := range envSets {
		result = append(result, envs...)
	}
	return result
}

// mergeVolumes concatenates volume sets, returning nil when every set is empty
// so the resulting PodSpec stays DeepEqual-stable with the no-TLS case.
func mergeVolumes(sets ...[]corev1.Volume) []corev1.Volume {
	var result []corev1.Volume
	for _, s := range sets {
		result = append(result, s...)
	}
	return result
}

// mergeVolumeMounts concatenates volume-mount sets, returning nil when every
// set is empty.
func mergeVolumeMounts(sets ...[]corev1.VolumeMount) []corev1.VolumeMount {
	var result []corev1.VolumeMount
	for _, s := range sets {
		result = append(result, s...)
	}
	return result
}
