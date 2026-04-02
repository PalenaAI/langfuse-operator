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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/bitkaio/langfuse-operator/api/v1alpha1"
	"github.com/bitkaio/langfuse-operator/internal/langfuse"
)

func ptrInt32(v int32) *int32 { return &v }
func ptrBool(v bool) *bool    { return &v }

func minimalInstance() *v1alpha1.LangfuseInstance {
	return &v1alpha1.LangfuseInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "langfuse",
		},
		Spec: v1alpha1.LangfuseInstanceSpec{
			Image: v1alpha1.ImageSpec{
				Tag: "3",
			},
			Auth: v1alpha1.AuthSpec{
				NextAuthUrl: "https://langfuse.example.com",
			},
		},
	}
}

func buildConfig(instance *v1alpha1.LangfuseInstance) *langfuse.Config {
	cfg, _ := langfuse.BuildConfig(instance)
	return cfg
}

func TestBuildWebDeployment_Minimal(t *testing.T) {
	instance := minimalInstance()
	config := buildConfig(instance)
	deploy := BuildWebDeployment(instance, config)

	if deploy.Name != "test-web" {
		t.Errorf("name = %q, want %q", deploy.Name, "test-web")
	}
	if deploy.Namespace != "langfuse" {
		t.Errorf("namespace = %q, want %q", deploy.Namespace, "langfuse")
	}
	if *deploy.Spec.Replicas != 1 {
		t.Errorf("replicas = %d, want %d", *deploy.Spec.Replicas, 1)
	}

	// Labels
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "langfuse",
		"app.kubernetes.io/instance":   "test",
		"app.kubernetes.io/component":  "web",
		"app.kubernetes.io/managed-by": "langfuse-operator",
		"app.kubernetes.io/part-of":    "langfuse",
		"langfuse.palena.ai/instance":  "test",
	}
	for k, v := range expectedLabels {
		if deploy.Labels[k] != v {
			t.Errorf("label %s = %q, want %q", k, deploy.Labels[k], v)
		}
	}

	// Container
	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("container count = %d, want 1", len(containers))
	}
	c := containers[0]
	if c.Image != "langfuse/langfuse:3" {
		t.Errorf("image = %q, want %q", c.Image, "langfuse/langfuse:3")
	}
	if len(c.Ports) != 1 || c.Ports[0].ContainerPort != 3000 {
		t.Errorf("port = %v, want 3000", c.Ports)
	}
	if c.LivenessProbe == nil || c.LivenessProbe.HTTPGet == nil {
		t.Error("liveness probe should be HTTP GET")
	} else if c.LivenessProbe.HTTPGet.Path != "/api/public/health" {
		t.Errorf("liveness path = %q, want %q", c.LivenessProbe.HTTPGet.Path, "/api/public/health")
	}
	if c.ReadinessProbe == nil || c.ReadinessProbe.HTTPGet == nil {
		t.Error("readiness probe should be HTTP GET")
	}

	// Env: should contain LANGFUSE_WORKER_ENABLED=false
	foundWorkerEnabled := false
	for _, e := range c.Env {
		if e.Name == "LANGFUSE_WORKER_ENABLED" {
			if e.Value != "false" {
				t.Errorf("LANGFUSE_WORKER_ENABLED = %q, want %q", e.Value, "false")
			}
			foundWorkerEnabled = true
		}
	}
	if !foundWorkerEnabled {
		t.Error("LANGFUSE_WORKER_ENABLED not found in env")
	}
}

func TestBuildWebDeployment_CustomReplicas(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Web.Replicas = ptrInt32(3)
	config := buildConfig(instance)
	deploy := BuildWebDeployment(instance, config)

	if *deploy.Spec.Replicas != 3 {
		t.Errorf("replicas = %d, want %d", *deploy.Spec.Replicas, 3)
	}
}

func TestBuildWebDeployment_Resources(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Web.Resources = &v1alpha1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}
	config := buildConfig(instance)
	deploy := BuildWebDeployment(instance, config)

	c := deploy.Spec.Template.Spec.Containers[0]
	if c.Resources.Requests.Cpu().String() != "500m" {
		t.Errorf("cpu request = %s, want 500m", c.Resources.Requests.Cpu())
	}
	if c.Resources.Limits.Memory().String() != "2Gi" {
		t.Errorf("memory limit = %s, want 2Gi", c.Resources.Limits.Memory())
	}
}

func TestBuildWebDeployment_SecurityContext(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Security = &v1alpha1.SecuritySpec{
		ReadOnlyRootFilesystem: ptrBool(true),
		RunAsNonRoot:           ptrBool(true),
	}
	config := buildConfig(instance)
	deploy := BuildWebDeployment(instance, config)

	sc := deploy.Spec.Template.Spec.Containers[0].SecurityContext
	if sc == nil {
		t.Fatal("security context should not be nil")
	}
	if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Error("readOnlyRootFilesystem should be true")
	}
	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Error("runAsNonRoot should be true")
	}
}

func TestBuildWebDeployment_ImagePullSecrets(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Image.PullSecrets = []corev1.LocalObjectReference{
		{Name: "registry-creds"},
	}
	config := buildConfig(instance)
	deploy := BuildWebDeployment(instance, config)

	if len(deploy.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Fatalf("imagePullSecrets count = %d, want 1", len(deploy.Spec.Template.Spec.ImagePullSecrets))
	}
	if deploy.Spec.Template.Spec.ImagePullSecrets[0].Name != "registry-creds" {
		t.Errorf("imagePullSecret = %q, want %q", deploy.Spec.Template.Spec.ImagePullSecrets[0].Name, "registry-creds")
	}
}

func TestBuildWebDeployment_CustomImage(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Image.Repository = "ghcr.io/custom/langfuse"
	instance.Spec.Image.Tag = "v3.1.0"
	config := buildConfig(instance)
	deploy := BuildWebDeployment(instance, config)

	c := deploy.Spec.Template.Spec.Containers[0]
	if c.Image != "ghcr.io/custom/langfuse:v3.1.0" {
		t.Errorf("image = %q, want %q", c.Image, "ghcr.io/custom/langfuse:v3.1.0")
	}
}
