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

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

const (
	testClickHouseName = "test-clickhouse"
	testPVCName        = "data"
)

func clickHouseInstance() *v1alpha1.LangfuseInstance {
	inst := minimalInstance()
	inst.Spec.ClickHouse = &v1alpha1.ClickHouseSpec{
		Managed: &v1alpha1.ManagedClickHouseSpec{},
	}
	return inst
}

func TestClickHouseName(t *testing.T) {
	instance := minimalInstance()
	if got := ClickHouseName(instance); got != testClickHouseName {
		t.Errorf("ClickHouseName() = %q, want %q", got, testClickHouseName)
	}
}

// ─── StatefulSet Tests ──────────────────────────────────────────────────────

func TestBuildClickHouseStatefulSet_Minimal(t *testing.T) {
	instance := clickHouseInstance()
	sts := BuildClickHouseStatefulSet(instance)

	if sts.Name != testClickHouseName {
		t.Errorf("name = %q, want %q", sts.Name, testClickHouseName)
	}
	if sts.Namespace != instance.Namespace {
		t.Errorf("namespace = %q, want %q", sts.Namespace, instance.Namespace)
	}
	if *sts.Spec.Replicas != 1 {
		t.Errorf("replicas = %d, want 1", *sts.Spec.Replicas)
	}
	if sts.Spec.ServiceName != testClickHouseName {
		t.Errorf("serviceName = %q, want %q", sts.Spec.ServiceName, testClickHouseName)
	}

	// Labels
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "langfuse",
		"app.kubernetes.io/instance":   "test",
		"app.kubernetes.io/component":  "clickhouse",
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

func TestBuildClickHouseStatefulSet_Image(t *testing.T) {
	instance := clickHouseInstance()
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	if c.Image != "clickhouse/clickhouse-server:24-alpine" {
		t.Errorf("image = %q, want %q", c.Image, "clickhouse/clickhouse-server:24-alpine")
	}
}

func TestBuildClickHouseStatefulSet_Ports(t *testing.T) {
	instance := clickHouseInstance()
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	if len(c.Ports) != 2 {
		t.Fatalf("port count = %d, want 2", len(c.Ports))
	}

	portMap := map[string]int32{}
	for _, p := range c.Ports {
		portMap[p.Name] = p.ContainerPort
	}
	if portMap["http"] != 8123 {
		t.Errorf("http port = %d, want 8123", portMap["http"])
	}
	if portMap["native"] != 9000 {
		t.Errorf("native port = %d, want 9000", portMap["native"])
	}
}

func TestBuildClickHouseStatefulSet_VolumeClaimDefaults(t *testing.T) {
	instance := clickHouseInstance()
	sts := BuildClickHouseStatefulSet(instance)

	if len(sts.Spec.VolumeClaimTemplates) != 1 {
		t.Fatalf("VolumeClaimTemplates count = %d, want 1", len(sts.Spec.VolumeClaimTemplates))
	}

	pvc := sts.Spec.VolumeClaimTemplates[0]
	if pvc.Name != testPVCName {
		t.Errorf("PVC name = %q, want %q", pvc.Name, testPVCName)
	}

	storageReq := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if storageReq.String() != "10Gi" {
		t.Errorf("storage = %s, want 10Gi", storageReq.String())
	}
	if pvc.Spec.StorageClassName != nil {
		t.Errorf("storageClassName = %q, want nil", *pvc.Spec.StorageClassName)
	}
}

func TestBuildClickHouseStatefulSet_CustomStorage(t *testing.T) {
	instance := clickHouseInstance()
	instance.Spec.ClickHouse.Managed.StorageSize = "100Gi"
	instance.Spec.ClickHouse.Managed.StorageClass = "fast-ssd"
	sts := BuildClickHouseStatefulSet(instance)

	pvc := sts.Spec.VolumeClaimTemplates[0]
	storageReq := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if storageReq.String() != "100Gi" {
		t.Errorf("storage = %s, want 100Gi", storageReq.String())
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast-ssd" {
		t.Errorf("storageClassName = %v, want %q", pvc.Spec.StorageClassName, "fast-ssd")
	}
}

func TestBuildClickHouseStatefulSet_CustomReplicas(t *testing.T) {
	instance := clickHouseInstance()
	instance.Spec.ClickHouse.Managed.Replicas = ptrInt32(3)
	sts := BuildClickHouseStatefulSet(instance)

	if *sts.Spec.Replicas != 3 {
		t.Errorf("replicas = %d, want 3", *sts.Spec.Replicas)
	}
}

func TestBuildClickHouseStatefulSet_DataVolumeMount(t *testing.T) {
	instance := clickHouseInstance()
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	found := false
	for _, vm := range c.VolumeMounts {
		if vm.Name == testPVCName && vm.MountPath == "/var/lib/clickhouse" {
			found = true
		}
	}
	if !found {
		t.Error("data volume mount at /var/lib/clickhouse not found")
	}
}

func TestBuildClickHouseStatefulSet_ConfigVolumeMount(t *testing.T) {
	instance := clickHouseInstance()
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	found := false
	for _, vm := range c.VolumeMounts {
		if vm.Name == "config" && vm.MountPath == "/etc/clickhouse-server/config.d" && vm.ReadOnly {
			found = true
		}
	}
	if !found {
		t.Error("config volume mount at /etc/clickhouse-server/config.d not found")
	}

	// Verify the config volume references the ConfigMap
	configVolFound := false
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == "config" && v.ConfigMap != nil && v.ConfigMap.Name == testClickHouseName {
			configVolFound = true
		}
	}
	if !configVolFound {
		t.Error("config volume referencing ConfigMap not found")
	}
}

func TestBuildClickHouseStatefulSet_Probes(t *testing.T) {
	instance := clickHouseInstance()
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]

	// Liveness probe
	if c.LivenessProbe == nil || c.LivenessProbe.HTTPGet == nil {
		t.Fatal("liveness probe should be HTTP GET")
	}
	if c.LivenessProbe.HTTPGet.Path != "/ping" {
		t.Errorf("liveness path = %q, want /ping", c.LivenessProbe.HTTPGet.Path)
	}
	if c.LivenessProbe.HTTPGet.Port.IntValue() != 8123 {
		t.Errorf("liveness port = %d, want 8123", c.LivenessProbe.HTTPGet.Port.IntValue())
	}

	// Readiness probe
	if c.ReadinessProbe == nil || c.ReadinessProbe.HTTPGet == nil {
		t.Fatal("readiness probe should be HTTP GET")
	}
	if c.ReadinessProbe.HTTPGet.Path != "/ping" {
		t.Errorf("readiness path = %q, want /ping", c.ReadinessProbe.HTTPGet.Path)
	}
	if c.ReadinessProbe.HTTPGet.Port.IntValue() != 8123 {
		t.Errorf("readiness port = %d, want 8123", c.ReadinessProbe.HTTPGet.Port.IntValue())
	}
}

func TestBuildClickHouseStatefulSet_DefaultAuthSecret(t *testing.T) {
	instance := clickHouseInstance()
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	expectedSecret := "test-generated-secrets"

	envMap := map[string]*corev1.EnvVarSource{}
	for _, e := range c.Env {
		envMap[e.Name] = e.ValueFrom
	}

	// CLICKHOUSE_USER
	if src, ok := envMap["CLICKHOUSE_USER"]; !ok {
		t.Error("CLICKHOUSE_USER not found in env")
	} else if src == nil || src.SecretKeyRef == nil {
		t.Error("CLICKHOUSE_USER should reference a secret")
	} else {
		if src.SecretKeyRef.Name != expectedSecret {
			t.Errorf("CLICKHOUSE_USER secret name = %q, want %q", src.SecretKeyRef.Name, expectedSecret)
		}
		if src.SecretKeyRef.Key != "clickhouse-username" {
			t.Errorf("CLICKHOUSE_USER secret key = %q, want %q", src.SecretKeyRef.Key, "clickhouse-username")
		}
	}

	// CLICKHOUSE_PASSWORD
	if src, ok := envMap["CLICKHOUSE_PASSWORD"]; !ok {
		t.Error("CLICKHOUSE_PASSWORD not found in env")
	} else if src == nil || src.SecretKeyRef == nil {
		t.Error("CLICKHOUSE_PASSWORD should reference a secret")
	} else {
		if src.SecretKeyRef.Name != expectedSecret {
			t.Errorf("CLICKHOUSE_PASSWORD secret name = %q, want %q", src.SecretKeyRef.Name, expectedSecret)
		}
		if src.SecretKeyRef.Key != "clickhouse-password" {
			t.Errorf("CLICKHOUSE_PASSWORD secret key = %q, want %q", src.SecretKeyRef.Key, "clickhouse-password")
		}
	}
}

func TestBuildClickHouseStatefulSet_CustomAuthSecret(t *testing.T) {
	instance := clickHouseInstance()
	instance.Spec.ClickHouse.Managed.Auth = &v1alpha1.ClickHouseAuthSpec{
		SecretRef: &v1alpha1.SecretKeysRef{
			Name: "my-ch-secret",
			Keys: map[string]string{
				"username": "ch-user",
				"password": "ch-pass",
			},
		},
	}
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	envMap := map[string]*corev1.EnvVarSource{}
	for _, e := range c.Env {
		envMap[e.Name] = e.ValueFrom
	}

	if src := envMap["CLICKHOUSE_USER"]; src == nil || src.SecretKeyRef == nil {
		t.Fatal("CLICKHOUSE_USER should reference a secret")
	} else {
		if src.SecretKeyRef.Name != "my-ch-secret" {
			t.Errorf("CLICKHOUSE_USER secret name = %q, want %q", src.SecretKeyRef.Name, "my-ch-secret")
		}
		if src.SecretKeyRef.Key != "ch-user" {
			t.Errorf("CLICKHOUSE_USER secret key = %q, want %q", src.SecretKeyRef.Key, "ch-user")
		}
	}

	if src := envMap["CLICKHOUSE_PASSWORD"]; src == nil || src.SecretKeyRef == nil {
		t.Fatal("CLICKHOUSE_PASSWORD should reference a secret")
	} else {
		if src.SecretKeyRef.Name != "my-ch-secret" {
			t.Errorf("CLICKHOUSE_PASSWORD secret name = %q, want %q", src.SecretKeyRef.Name, "my-ch-secret")
		}
		if src.SecretKeyRef.Key != "ch-pass" {
			t.Errorf("CLICKHOUSE_PASSWORD secret key = %q, want %q", src.SecretKeyRef.Key, "ch-pass")
		}
	}
}

// ─── Resource Preset Tests ──────────────────────────────────────────────────

func TestBuildClickHouseStatefulSet_ResourcePresetSmall(t *testing.T) {
	instance := clickHouseInstance()
	instance.Spec.ClickHouse.Managed.Resources = &v1alpha1.ClickHouseResourceSpec{
		Preset: "small",
	}
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceCPU, "1")
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceMemory, "2Gi")
	assertResourceQuantity(t, c.Resources.Limits, corev1.ResourceCPU, "1")
	assertResourceQuantity(t, c.Resources.Limits, corev1.ResourceMemory, "2Gi")
}

func TestBuildClickHouseStatefulSet_ResourcePresetMedium(t *testing.T) {
	instance := clickHouseInstance()
	instance.Spec.ClickHouse.Managed.Resources = &v1alpha1.ClickHouseResourceSpec{
		Preset: "medium",
	}
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceCPU, "2")
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceMemory, "8Gi")
	assertResourceQuantity(t, c.Resources.Limits, corev1.ResourceCPU, "2")
	assertResourceQuantity(t, c.Resources.Limits, corev1.ResourceMemory, "8Gi")
}

func TestBuildClickHouseStatefulSet_ResourcePresetLarge(t *testing.T) {
	instance := clickHouseInstance()
	instance.Spec.ClickHouse.Managed.Resources = &v1alpha1.ClickHouseResourceSpec{
		Preset: "large",
	}
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceCPU, "4")
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceMemory, "16Gi")
	assertResourceQuantity(t, c.Resources.Limits, corev1.ResourceCPU, "4")
	assertResourceQuantity(t, c.Resources.Limits, corev1.ResourceMemory, "16Gi")
}

func TestBuildClickHouseStatefulSet_ResourcePresetDefault(t *testing.T) {
	// No resources spec at all should default to small
	instance := clickHouseInstance()
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceCPU, "1")
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceMemory, "2Gi")
}

func TestBuildClickHouseStatefulSet_ResourceCustom(t *testing.T) {
	instance := clickHouseInstance()
	instance.Spec.ClickHouse.Managed.Resources = &v1alpha1.ClickHouseResourceSpec{
		Preset: "custom",
		Custom: &v1alpha1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3"),
				corev1.ResourceMemory: resource.MustParse("12Gi"),
			},
		},
	}
	sts := BuildClickHouseStatefulSet(instance)

	c := sts.Spec.Template.Spec.Containers[0]
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceCPU, "500m")
	assertResourceQuantity(t, c.Resources.Requests, corev1.ResourceMemory, "1Gi")
	assertResourceQuantity(t, c.Resources.Limits, corev1.ResourceCPU, "3")
	assertResourceQuantity(t, c.Resources.Limits, corev1.ResourceMemory, "12Gi")
}

// ─── Service Tests ──────────────────────────────────────────────────────────

func TestBuildClickHouseService_Minimal(t *testing.T) {
	instance := clickHouseInstance()
	svc := BuildClickHouseService(instance)

	if svc.Name != testClickHouseName {
		t.Errorf("name = %q, want %q", svc.Name, testClickHouseName)
	}
	if svc.Namespace != instance.Namespace {
		t.Errorf("namespace = %q, want %q", svc.Namespace, instance.Namespace)
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("type = %q, want ClusterIP", svc.Spec.Type)
	}

	if len(svc.Spec.Ports) != 2 {
		t.Fatalf("port count = %d, want 2", len(svc.Spec.Ports))
	}

	portMap := map[string]int32{}
	for _, p := range svc.Spec.Ports {
		portMap[p.Name] = p.Port
	}
	if portMap["http"] != 8123 {
		t.Errorf("http port = %d, want 8123", portMap["http"])
	}
	if portMap["native"] != 9000 {
		t.Errorf("native port = %d, want 9000", portMap["native"])
	}

	// Selector labels
	if svc.Spec.Selector["app.kubernetes.io/component"] != "clickhouse" {
		t.Errorf("selector component = %q, want clickhouse", svc.Spec.Selector["app.kubernetes.io/component"])
	}
}

// ─── ConfigMap Tests ────────────────────────────────────────────────────────

func TestBuildClickHouseConfigMap(t *testing.T) {
	instance := clickHouseInstance()
	cm := BuildClickHouseConfigMap(instance)

	if cm.Name != testClickHouseName {
		t.Errorf("name = %q, want %q", cm.Name, testClickHouseName)
	}
	if cm.Namespace != instance.Namespace {
		t.Errorf("namespace = %q, want %q", cm.Namespace, instance.Namespace)
	}
	if _, ok := cm.Data["config.xml"]; !ok {
		t.Error("config.xml not found in ConfigMap data")
	}
	if _, ok := cm.Data["users.xml"]; !ok {
		t.Error("users.xml not found in ConfigMap data")
	}
	if cm.Labels["app.kubernetes.io/component"] != "clickhouse" {
		t.Errorf("component label = %q, want clickhouse", cm.Labels["app.kubernetes.io/component"])
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func assertResourceQuantity(t *testing.T, list corev1.ResourceList, name corev1.ResourceName, expected string) {
	t.Helper()
	qty := list[name]
	expectedQty := resource.MustParse(expected)
	if qty.Cmp(expectedQty) != 0 {
		t.Errorf("%s = %s, want %s", name, qty.String(), expected)
	}
}
