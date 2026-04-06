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

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

const (
	testRedisName      = "test-redis"
	testRedisNamespace = "langfuse"
)

func TestRedisName(t *testing.T) {
	instance := minimalInstance()
	if got := RedisName(instance); got != testRedisName {
		t.Errorf("RedisName() = %q, want %q", got, testRedisName)
	}
}

func TestBuildRedisStatefulSet_Metadata(t *testing.T) {
	instance := minimalInstance()
	sts := BuildRedisStatefulSet(instance)

	if sts.Name != testRedisName {
		t.Errorf("name = %q, want %q", sts.Name, testRedisName)
	}
	if sts.Namespace != testRedisNamespace {
		t.Errorf("namespace = %q, want %q", sts.Namespace, testRedisNamespace)
	}
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 1 {
		t.Errorf("replicas = %v, want 1", sts.Spec.Replicas)
	}

	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "langfuse",
		"app.kubernetes.io/instance":   "test",
		"app.kubernetes.io/component":  "redis",
		"app.kubernetes.io/managed-by": "langfuse-operator",
		"app.kubernetes.io/part-of":    "langfuse",
		"langfuse.palena.ai/instance":  "test",
	}
	for k, v := range expectedLabels {
		if sts.Labels[k] != v {
			t.Errorf("label %s = %q, want %q", k, sts.Labels[k], v)
		}
	}
}

func TestBuildRedisStatefulSet_Container(t *testing.T) {
	instance := minimalInstance()
	sts := BuildRedisStatefulSet(instance)

	containers := sts.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("container count = %d, want 1", len(containers))
	}
	c := containers[0]

	if c.Image != "redis:7-alpine" {
		t.Errorf("image = %q, want %q", c.Image, "redis:7-alpine")
	}
	if len(c.Ports) != 1 || c.Ports[0].ContainerPort != 6379 {
		t.Errorf("port = %v, want 6379", c.Ports)
	}
	if c.Ports[0].Name != "redis" {
		t.Errorf("port name = %q, want %q", c.Ports[0].Name, "redis")
	}

	expectedCmd := []string{"redis-server", "--requirepass", "$(REDIS_PASSWORD)", "--appendonly", "yes"}
	if len(c.Command) != len(expectedCmd) {
		t.Fatalf("command length = %d, want %d", len(c.Command), len(expectedCmd))
	}
	for i, v := range expectedCmd {
		if c.Command[i] != v {
			t.Errorf("command[%d] = %q, want %q", i, c.Command[i], v)
		}
	}

	// Env
	if len(c.Env) != 1 {
		t.Fatalf("env count = %d, want 1", len(c.Env))
	}
	env := c.Env[0]
	if env.Name != "REDIS_PASSWORD" {
		t.Errorf("env name = %q, want %q", env.Name, "REDIS_PASSWORD")
	}
	if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
		t.Fatal("REDIS_PASSWORD should reference a secret")
	}
	if env.ValueFrom.SecretKeyRef.Name != "test-generated-secrets" {
		t.Errorf("secret name = %q, want %q", env.ValueFrom.SecretKeyRef.Name, "test-generated-secrets")
	}
	if env.ValueFrom.SecretKeyRef.Key != "redis-password" {
		t.Errorf("secret key = %q, want %q", env.ValueFrom.SecretKeyRef.Key, "redis-password")
	}

	// Probes
	if c.LivenessProbe == nil || c.LivenessProbe.Exec == nil {
		t.Fatal("liveness probe should be exec-based")
	}
	if c.ReadinessProbe == nil || c.ReadinessProbe.Exec == nil {
		t.Fatal("readiness probe should be exec-based")
	}
}

func TestBuildRedisStatefulSet_VolumeAndStorage(t *testing.T) {
	instance := minimalInstance()
	sts := BuildRedisStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	if len(c.VolumeMounts) != 1 {
		t.Fatalf("volumeMount count = %d, want 1", len(c.VolumeMounts))
	}
	if c.VolumeMounts[0].MountPath != "/data" {
		t.Errorf("mount path = %q, want %q", c.VolumeMounts[0].MountPath, "/data")
	}
	if c.VolumeMounts[0].Name != "data" {
		t.Errorf("mount name = %q, want %q", c.VolumeMounts[0].Name, "data")
	}

	if len(sts.Spec.VolumeClaimTemplates) != 1 {
		t.Fatalf("VolumeClaimTemplates count = %d, want 1", len(sts.Spec.VolumeClaimTemplates))
	}
	pvc := sts.Spec.VolumeClaimTemplates[0]
	if pvc.Name != "data" {
		t.Errorf("PVC name = %q, want %q", pvc.Name, "data")
	}
	storageReq := pvc.Spec.Resources.Requests["storage"]
	if storageReq.String() != "1Gi" {
		t.Errorf("storage size = %q, want %q", storageReq.String(), "1Gi")
	}
}

func TestBuildRedisStatefulSet_CustomStorageSize(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Redis = &v1alpha1.RedisSpec{
		Managed: &v1alpha1.ManagedRedisSpec{
			StorageSize: "10Gi",
		},
	}
	sts := BuildRedisStatefulSet(instance)

	pvc := sts.Spec.VolumeClaimTemplates[0]
	storageReq := pvc.Spec.Resources.Requests["storage"]
	if storageReq.String() != "10Gi" {
		t.Errorf("storage size = %q, want %q", storageReq.String(), "10Gi")
	}
}

func TestBuildRedisStatefulSet_ServiceName(t *testing.T) {
	instance := minimalInstance()
	sts := BuildRedisStatefulSet(instance)

	if sts.Spec.ServiceName != testRedisName {
		t.Errorf("serviceName = %q, want %q", sts.Spec.ServiceName, testRedisName)
	}
}

func TestBuildRedisService(t *testing.T) {
	instance := minimalInstance()
	svc := BuildRedisService(instance)

	// Name and namespace
	if svc.Name != testRedisName {
		t.Errorf("name = %q, want %q", svc.Name, testRedisName)
	}
	if svc.Namespace != testRedisNamespace {
		t.Errorf("namespace = %q, want %q", svc.Namespace, "langfuse")
	}

	// Type
	if svc.Spec.Type != "ClusterIP" {
		t.Errorf("type = %q, want %q", svc.Spec.Type, "ClusterIP")
	}

	// Port
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("port count = %d, want 1", len(svc.Spec.Ports))
	}
	port := svc.Spec.Ports[0]
	if port.Name != "redis" {
		t.Errorf("port name = %q, want %q", port.Name, "redis")
	}
	if port.Port != 6379 {
		t.Errorf("port = %d, want %d", port.Port, 6379)
	}

	// Selector labels
	expectedSelector := map[string]string{
		"app.kubernetes.io/name":      "langfuse",
		"app.kubernetes.io/instance":  "test",
		"app.kubernetes.io/component": "redis",
	}
	for k, v := range expectedSelector {
		if svc.Spec.Selector[k] != v {
			t.Errorf("selector %s = %q, want %q", k, svc.Spec.Selector[k], v)
		}
	}
}
