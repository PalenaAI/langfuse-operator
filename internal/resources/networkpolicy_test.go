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
	networkingv1 "k8s.io/api/networking/v1"
)

func TestBuildWebNetworkPolicy(t *testing.T) {
	instance := minimalInstance()
	np := BuildWebNetworkPolicy(instance)

	if np.Name != "test-web-netpol" {
		t.Errorf("name = %q, want %q", np.Name, "test-web-netpol")
	}
	if np.Namespace != instance.Namespace {
		t.Errorf("namespace = %q, want %q", np.Namespace, instance.Namespace)
	}

	// Should select web pods
	if np.Spec.PodSelector.MatchLabels["app.kubernetes.io/component"] != "web" {
		t.Errorf("pod selector component = %q, want %q", np.Spec.PodSelector.MatchLabels["app.kubernetes.io/component"], "web")
	}

	// Should have both Ingress and Egress policy types
	if len(np.Spec.PolicyTypes) != 2 {
		t.Fatalf("policy types count = %d, want 2", len(np.Spec.PolicyTypes))
	}
	hasIngress := false
	hasEgress := false
	for _, pt := range np.Spec.PolicyTypes {
		if pt == networkingv1.PolicyTypeIngress {
			hasIngress = true
		}
		if pt == networkingv1.PolicyTypeEgress {
			hasEgress = true
		}
	}
	if !hasIngress {
		t.Error("missing PolicyTypeIngress")
	}
	if !hasEgress {
		t.Error("missing PolicyTypeEgress")
	}

	// Should have ingress rule allowing port 3000
	if len(np.Spec.Ingress) != 1 {
		t.Fatalf("ingress rules count = %d, want 1", len(np.Spec.Ingress))
	}
	if len(np.Spec.Ingress[0].Ports) != 1 {
		t.Fatalf("ingress ports count = %d, want 1", len(np.Spec.Ingress[0].Ports))
	}
	if np.Spec.Ingress[0].Ports[0].Port.IntValue() != 3000 {
		t.Errorf("ingress port = %d, want 3000", np.Spec.Ingress[0].Ports[0].Port.IntValue())
	}

	// Should have egress rules (data stores + DNS)
	if len(np.Spec.Egress) != 2 {
		t.Fatalf("egress rules count = %d, want 2", len(np.Spec.Egress))
	}

	// Verify DNS egress includes UDP
	dnsRule := np.Spec.Egress[1]
	hasUDP := false
	for _, p := range dnsRule.Ports {
		if *p.Protocol == corev1.ProtocolUDP && p.Port.IntValue() == 53 {
			hasUDP = true
		}
	}
	if !hasUDP {
		t.Error("DNS egress rule missing UDP port 53")
	}
}

func TestBuildWorkerNetworkPolicy(t *testing.T) {
	instance := minimalInstance()
	np := BuildWorkerNetworkPolicy(instance)

	if np.Name != "test-worker-netpol" {
		t.Errorf("name = %q, want %q", np.Name, "test-worker-netpol")
	}

	// Should select worker pods
	if np.Spec.PodSelector.MatchLabels["app.kubernetes.io/component"] != "worker" {
		t.Errorf("pod selector component = %q, want %q", np.Spec.PodSelector.MatchLabels["app.kubernetes.io/component"], "worker")
	}

	// Worker should have NO ingress rules (empty slice = deny all ingress)
	if len(np.Spec.Ingress) != 0 {
		t.Errorf("worker ingress rules count = %d, want 0 (deny all)", len(np.Spec.Ingress))
	}

	// Should still have egress rules
	if len(np.Spec.Egress) != 2 {
		t.Fatalf("egress rules count = %d, want 2", len(np.Spec.Egress))
	}

	// Verify PostgreSQL port is in data store egress
	dsRule := np.Spec.Egress[0]
	hasPG := false
	for _, p := range dsRule.Ports {
		if *p.Protocol == corev1.ProtocolTCP && p.Port.IntValue() == 5432 {
			hasPG = true
		}
	}
	if !hasPG {
		t.Error("data store egress missing PostgreSQL port 5432")
	}
}
