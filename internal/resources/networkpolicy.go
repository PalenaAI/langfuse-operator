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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// BuildWebNetworkPolicy constructs a NetworkPolicy for the Langfuse Web component.
// It allows ingress on port 3000 from any source, and egress to PostgreSQL (5432),
// ClickHouse (8123, 9000), Redis (6379), and DNS (53).
func BuildWebNetworkPolicy(instance *v1alpha1.LangfuseInstance) *networkingv1.NetworkPolicy {
	labels := CommonLabels(instance, "web")
	selectorLabels := SelectorLabels(instance, "web")

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebName(instance) + "-netpol",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: protocolPtr(corev1.ProtocolTCP),
							Port:     intstrPtr(3000),
						},
					},
				},
			},
			Egress: commonEgressRules(),
		},
	}
}

// BuildWorkerNetworkPolicy constructs a NetworkPolicy for the Langfuse Worker component.
// It allows no ingress (worker exposes no ports), and egress to PostgreSQL (5432),
// ClickHouse (8123, 9000), Redis (6379), and DNS (53).
func BuildWorkerNetworkPolicy(instance *v1alpha1.LangfuseInstance) *networkingv1.NetworkPolicy {
	labels := CommonLabels(instance, "worker")
	selectorLabels := SelectorLabels(instance, "worker")

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkerName(instance) + "-netpol",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			// No ingress rules — worker has no inbound traffic
			Ingress: []networkingv1.NetworkPolicyIngressRule{},
			Egress:  commonEgressRules(),
		},
	}
}

// commonEgressRules returns egress rules shared by web and worker:
// PostgreSQL (5432), ClickHouse HTTP (8123), ClickHouse native (9000),
// Redis (6379), HTTPS (443 for blob storage / external APIs), and DNS (53 UDP+TCP).
func commonEgressRules() []networkingv1.NetworkPolicyEgressRule {
	return []networkingv1.NetworkPolicyEgressRule{
		{
			// Data stores
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(5432)}, // PostgreSQL
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(8123)}, // ClickHouse HTTP
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(9000)}, // ClickHouse native
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(6379)}, // Redis
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(443)},  // HTTPS (blob storage, LLM APIs)
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(9000)}, // MinIO S3 (common port)
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(3000)}, // Web ↔ Worker internal
			},
		},
		{
			// DNS
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: protocolPtr(corev1.ProtocolUDP), Port: intstrPtr(53)},
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(53)},
			},
		},
	}
}

func protocolPtr(p corev1.Protocol) *corev1.Protocol {
	return &p
}

func intstrPtr(port int) *intstr.IntOrString {
	v := intstr.FromInt32(int32(port))
	return &v
}
