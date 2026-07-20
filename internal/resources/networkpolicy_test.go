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

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
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

// egressPortSet collects the allowed egress ports for a protocol.
func egressPortSet(np *networkingv1.NetworkPolicy, proto corev1.Protocol) map[int32]bool {
	out := map[int32]bool{}
	for _, rule := range np.Spec.Egress {
		for _, p := range rule.Ports {
			if p.Protocol != nil && *p.Protocol == proto && p.Port != nil {
				out[p.Port.IntVal] = true
			}
		}
	}
	return out
}

// Regression guard: the operator's own default NetworkPolicy must not block the
// TLS endpoints its datastore-TLS documentation tells users to configure.
// Enabling clickhouse/redis TLS previously produced a silent connection timeout.
func TestNetworkPolicy_AllowsDatastoreTLSPorts(t *testing.T) {
	for name, np := range map[string]*networkingv1.NetworkPolicy{
		"web":    BuildWebNetworkPolicy(minimalInstance()),
		"worker": BuildWorkerNetworkPolicy(minimalInstance()),
	} {
		ports := egressPortSet(np, corev1.ProtocolTCP)
		for _, tc := range []struct {
			port int32
			what string
		}{
			{5432, "PostgreSQL"},
			{8123, "ClickHouse HTTP"},
			{8443, "ClickHouse HTTPS (TLS)"},
			{9000, "ClickHouse native / MinIO"},
			{9440, "ClickHouse native secure (TLS)"},
			{6379, "Redis"},
			{6380, "Redis TLS"},
			{443, "HTTPS"},
			{3000, "web<->worker"},
		} {
			if !ports[tc.port] {
				t.Errorf("%s netpol blocks port %d (%s)", name, tc.port, tc.what)
			}
		}
	}
}

func TestNetworkPolicy_AllowsDNS(t *testing.T) {
	np := BuildWebNetworkPolicy(minimalInstance())
	if !egressPortSet(np, corev1.ProtocolUDP)[53] {
		t.Error("UDP DNS blocked")
	}
	if !egressPortSet(np, corev1.ProtocolTCP)[53] {
		t.Error("TCP DNS blocked")
	}
}

func TestNetworkPolicy_ExtraEgressPorts(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Security = &v1alpha1.SecuritySpec{
		NetworkPolicy: &v1alpha1.NetworkPolicySpec{
			ExtraEgressPorts: []v1alpha1.NetworkPolicyPort{
				{Port: 15432}, // Postgres behind a pooler on a custom port
				{Port: 5432},  // duplicate of a default — must be skipped
				{Port: 1514, Protocol: corev1.ProtocolUDP}, // e.g. syslog sidecar
			},
		},
	}

	np := BuildWorkerNetworkPolicy(instance)
	tcp := egressPortSet(np, corev1.ProtocolTCP)
	if !tcp[15432] {
		t.Error("extra TCP port 15432 not allowed")
	}
	if !egressPortSet(np, corev1.ProtocolUDP)[1514] {
		t.Error("extra UDP port 1514 not allowed")
	}

	// The duplicate must not be emitted twice.
	count := 0
	for _, rule := range np.Spec.Egress {
		for _, p := range rule.Ports {
			if p.Port != nil && p.Port.IntVal == 5432 && p.Protocol != nil && *p.Protocol == corev1.ProtocolTCP {
				count++
			}
		}
	}
	if count != 1 {
		t.Errorf("port 5432 appears %d times, want 1 (duplicate should be skipped)", count)
	}
}

func TestNetworkPolicy_NoExtraPortsWhenSecurityUnset(t *testing.T) {
	np := BuildWebNetworkPolicy(minimalInstance())
	if got := len(egressPortSet(np, corev1.ProtocolTCP)); got != len(defaultEgressPorts)+1 {
		// +1 for TCP DNS (53), which lives in the second rule.
		t.Errorf("TCP port count = %d, want %d defaults + DNS", got, len(defaultEgressPorts))
	}
}
