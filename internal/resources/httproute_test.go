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

func TestBuildHTTPRoute_Basic(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.GatewayAPI = &v1alpha1.GatewayAPISpec{
		Enabled: true,
		GatewayRef: v1alpha1.GatewayRef{
			Name: "my-gateway",
		},
		Hostname: testHostname,
	}

	route := BuildHTTPRoute(instance)

	if route.GetName() != testWebName {
		t.Errorf("name = %q, want %q", route.GetName(), testWebName)
	}
	if route.GetNamespace() != instance.Namespace {
		t.Errorf("namespace = %q, want %q", route.GetNamespace(), instance.Namespace)
	}
	if route.GetKind() != "HTTPRoute" {
		t.Errorf("kind = %q, want %q", route.GetKind(), "HTTPRoute")
	}
	if route.GetAPIVersion() != "gateway.networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want %q", route.GetAPIVersion(), "gateway.networking.k8s.io/v1")
	}

	spec := route.Object["spec"].(map[string]interface{})

	// Hostnames
	hostnames := spec["hostnames"].([]interface{})
	if len(hostnames) != 1 || hostnames[0] != testHostname {
		t.Errorf("hostnames = %v, want [langfuse.example.com]", hostnames)
	}

	// Parent refs
	parentRefs := spec["parentRefs"].([]interface{})
	if len(parentRefs) != 1 {
		t.Fatalf("parentRefs count = %d, want 1", len(parentRefs))
	}
	parent := parentRefs[0].(map[string]interface{})
	if parent["name"] != "my-gateway" {
		t.Errorf("parentRef name = %q, want %q", parent["name"], "my-gateway")
	}

	// Rules
	rules := spec["rules"].([]interface{})
	if len(rules) != 1 {
		t.Fatalf("rules count = %d, want 1", len(rules))
	}
	rule := rules[0].(map[string]interface{})

	// Backend refs
	backendRefs := rule["backendRefs"].([]interface{})
	if len(backendRefs) != 1 {
		t.Fatalf("backendRefs count = %d, want 1", len(backendRefs))
	}
	backend := backendRefs[0].(map[string]interface{})
	if backend["name"] != testWebName {
		t.Errorf("backend name = %q, want %q", backend["name"], testWebName)
	}
	if backend["port"] != int64(3000) {
		t.Errorf("backend port = %v, want 3000", backend["port"])
	}
}

func TestBuildHTTPRoute_NoHostname(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.GatewayAPI = &v1alpha1.GatewayAPISpec{
		Enabled: true,
		GatewayRef: v1alpha1.GatewayRef{
			Name: "my-gateway",
		},
	}

	route := BuildHTTPRoute(instance)
	spec := route.Object["spec"].(map[string]interface{})

	if _, ok := spec["hostnames"]; ok {
		t.Error("hostnames should not be set when empty")
	}
}

func TestBuildHTTPRoute_GatewayRefWithNamespace(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.GatewayAPI = &v1alpha1.GatewayAPISpec{
		Enabled: true,
		GatewayRef: v1alpha1.GatewayRef{
			Name:        "shared-gateway",
			Namespace:   "gateway-system",
			SectionName: "https",
		},
		Hostname: testHostname,
	}

	route := BuildHTTPRoute(instance)
	spec := route.Object["spec"].(map[string]interface{})

	parentRefs := spec["parentRefs"].([]interface{})
	parent := parentRefs[0].(map[string]interface{})
	if parent["name"] != "shared-gateway" {
		t.Errorf("parentRef name = %q, want %q", parent["name"], "shared-gateway")
	}
	if parent["namespace"] != "gateway-system" {
		t.Errorf("parentRef namespace = %q, want %q", parent["namespace"], "gateway-system")
	}
	if parent["sectionName"] != "https" {
		t.Errorf("parentRef sectionName = %q, want %q", parent["sectionName"], "https")
	}
}

func TestBuildHTTPRoute_Annotations(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.GatewayAPI = &v1alpha1.GatewayAPISpec{
		Enabled: true,
		GatewayRef: v1alpha1.GatewayRef{
			Name: "my-gateway",
		},
		Annotations: map[string]string{
			"custom.io/timeout": "60s",
		},
	}

	route := BuildHTTPRoute(instance)

	annotations := route.GetAnnotations()
	if annotations["custom.io/timeout"] != "60s" {
		t.Errorf("annotation missing or wrong")
	}
}
