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

// BuildWebNetworkPolicy constructs a NetworkPolicy for the Langfuse Web
// component. It allows ingress on port 3000 from any source, plus the shared
// datastore/DNS egress rules (see commonEgressRules).
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
			Egress: commonEgressRules(instance),
		},
	}
}

// BuildWorkerNetworkPolicy constructs a NetworkPolicy for the Langfuse Worker
// component. It allows no ingress (the worker exposes no ports), plus the
// shared datastore/DNS egress rules (see commonEgressRules).
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
			Egress:  commonEgressRules(instance),
		},
	}
}

// defaultEgressPorts are the well-known datastore ports the operator opens by
// default. Both the plaintext and TLS ports of each store are included: the
// real port lives in a connection Secret the operator does not read, so it
// cannot narrow this down per instance. Anything unusual goes through
// spec.security.networkPolicy.extraEgressPorts.
var defaultEgressPorts = []int{
	5432, // PostgreSQL
	8123, // ClickHouse HTTP
	8443, // ClickHouse HTTPS (TLS)
	9000, // ClickHouse native / MinIO S3
	9440, // ClickHouse native secure (TLS)
	6379, // Redis
	6380, // Redis TLS (conventional)
	443,  // HTTPS — blob storage, LLM APIs, OIDC providers
	3000, // Web ↔ Worker internal
}

// commonEgressRules returns the egress rules shared by web and worker: the
// datastore ports above, any operator-configured extra ports, and DNS.
func commonEgressRules(instance *v1alpha1.LangfuseInstance) []networkingv1.NetworkPolicyEgressRule {
	ports := make([]networkingv1.NetworkPolicyPort, 0, len(defaultEgressPorts))
	for _, p := range defaultEgressPorts {
		ports = append(ports, networkingv1.NetworkPolicyPort{
			Protocol: protocolPtr(corev1.ProtocolTCP),
			Port:     intstrPtr(p),
		})
	}
	ports = append(ports, extraEgressPorts(instance)...)

	return []networkingv1.NetworkPolicyEgressRule{
		{Ports: ports},
		{
			// DNS
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: protocolPtr(corev1.ProtocolUDP), Port: intstrPtr(53)},
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(53)},
			},
		},
	}
}

// extraEgressPorts renders spec.security.networkPolicy.extraEgressPorts,
// skipping any that duplicate a default port so the rule stays tidy.
func extraEgressPorts(instance *v1alpha1.LangfuseInstance) []networkingv1.NetworkPolicyPort {
	if instance.Spec.Security == nil || instance.Spec.Security.NetworkPolicy == nil {
		return nil
	}

	defaults := make(map[int]bool, len(defaultEgressPorts))
	for _, p := range defaultEgressPorts {
		defaults[p] = true
	}

	extras := instance.Spec.Security.NetworkPolicy.ExtraEgressPorts
	out := make([]networkingv1.NetworkPolicyPort, 0, len(extras))
	for _, extra := range extras {
		protocol := extra.Protocol
		if protocol == "" {
			protocol = corev1.ProtocolTCP
		}
		if protocol == corev1.ProtocolTCP && defaults[int(extra.Port)] {
			continue
		}
		out = append(out, networkingv1.NetworkPolicyPort{
			Protocol: protocolPtr(protocol),
			Port:     intstrPtr(int(extra.Port)),
		})
	}
	return out
}

func protocolPtr(p corev1.Protocol) *corev1.Protocol {
	return &p
}

func intstrPtr(port int) *intstr.IntOrString {
	v := intstr.FromInt32(int32(port))
	return &v
}
