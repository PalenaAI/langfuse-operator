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

func TestBuildRoute_Basic(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Route = &v1alpha1.RouteSpec{
		Enabled: true,
		Host:    "langfuse.apps.example.com",
	}

	route := BuildRoute(instance)

	if route.GetName() != testWebName {
		t.Errorf("name = %q, want %q", route.GetName(), testWebName)
	}
	if route.GetNamespace() != instance.Namespace {
		t.Errorf("namespace = %q, want %q", route.GetNamespace(), instance.Namespace)
	}
	if route.GetKind() != "Route" {
		t.Errorf("kind = %q, want %q", route.GetKind(), "Route")
	}
	if route.GetAPIVersion() != "route.openshift.io/v1" {
		t.Errorf("apiVersion = %q, want %q", route.GetAPIVersion(), "route.openshift.io/v1")
	}

	spec := route.Object["spec"].(map[string]interface{})

	// Host
	if spec["host"] != "langfuse.apps.example.com" {
		t.Errorf("host = %q, want %q", spec["host"], "langfuse.apps.example.com")
	}

	// Service target
	to := spec["to"].(map[string]interface{})
	if to["name"] != testWebName {
		t.Errorf("target service = %q, want %q", to["name"], testWebName)
	}
	if to["kind"] != "Service" {
		t.Errorf("target kind = %q, want %q", to["kind"], "Service")
	}

	// TLS
	tls := spec["tls"].(map[string]interface{})
	if tls["termination"] != "edge" {
		t.Errorf("tls termination = %q, want %q", tls["termination"], "edge")
	}
}

func TestBuildRoute_NoHost(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Route = &v1alpha1.RouteSpec{
		Enabled: true,
	}

	route := BuildRoute(instance)
	spec := route.Object["spec"].(map[string]interface{})

	// Host should not be set (OpenShift will auto-generate)
	if _, ok := spec["host"]; ok {
		t.Error("host should not be set when empty")
	}
}

func TestBuildRoute_Annotations(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Route = &v1alpha1.RouteSpec{
		Enabled: true,
		Annotations: map[string]string{
			"haproxy.router.openshift.io/timeout": "60s",
		},
	}

	route := BuildRoute(instance)

	annotations := route.GetAnnotations()
	if annotations["haproxy.router.openshift.io/timeout"] != "60s" {
		t.Errorf("annotation missing or wrong")
	}
}
