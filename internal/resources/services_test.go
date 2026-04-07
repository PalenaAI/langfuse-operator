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
)

func TestBuildWebService(t *testing.T) {
	instance := minimalInstance()
	svc := BuildWebService(instance)

	if svc.Name != testWebName {
		t.Errorf("name = %q, want %q", svc.Name, testWebName)
	}
	if svc.Namespace != "langfuse" {
		t.Errorf("namespace = %q, want %q", svc.Namespace, "langfuse")
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("type = %q, want %q", svc.Spec.Type, corev1.ServiceTypeClusterIP)
	}

	// Selector
	expectedSelector := map[string]string{
		"app.kubernetes.io/name":      "langfuse",
		"app.kubernetes.io/instance":  "test",
		"app.kubernetes.io/component": "web",
	}
	for k, v := range expectedSelector {
		if svc.Spec.Selector[k] != v {
			t.Errorf("selector %s = %q, want %q", k, svc.Spec.Selector[k], v)
		}
	}

	// Ports
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("port count = %d, want 1", len(svc.Spec.Ports))
	}
	p := svc.Spec.Ports[0]
	if p.Port != 3000 {
		t.Errorf("port = %d, want %d", p.Port, 3000)
	}
	if p.Protocol != corev1.ProtocolTCP {
		t.Errorf("protocol = %q, want %q", p.Protocol, corev1.ProtocolTCP)
	}

	// Labels
	if svc.Labels["app.kubernetes.io/component"] != "web" {
		t.Errorf("component label = %q, want %q", svc.Labels["app.kubernetes.io/component"], "web")
	}
}
