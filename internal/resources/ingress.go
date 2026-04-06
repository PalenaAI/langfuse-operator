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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// IngressName returns the name for the Ingress resource.
func IngressName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-web"
}

// BuildIngress constructs the desired Ingress for the Langfuse Web component.
func BuildIngress(instance *v1alpha1.LangfuseInstance) *networkingv1.Ingress {
	spec := instance.Spec.Ingress
	labels := CommonLabels(instance, "web")

	annotations := make(map[string]string)
	for k, v := range spec.Annotations {
		annotations[k] = v
	}

	// cert-manager annotation
	if spec.TLS != nil && spec.TLS.CertManager != nil {
		kind := spec.TLS.CertManager.IssuerRef.Kind
		if kind == "" {
			kind = "ClusterIssuer"
		}
		if kind == "ClusterIssuer" {
			annotations["cert-manager.io/cluster-issuer"] = spec.TLS.CertManager.IssuerRef.Name
		} else {
			annotations["cert-manager.io/issuer"] = spec.TLS.CertManager.IssuerRef.Name
		}
	}

	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        IngressName(instance),
			Namespace:   instance.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: spec.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: WebServiceName(instance),
											Port: networkingv1.ServiceBackendPort{
												Number: 3000,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if spec.ClassName != "" {
		ingress.Spec.IngressClassName = &spec.ClassName
	}

	// TLS
	if spec.TLS != nil && spec.TLS.Enabled {
		secretName := spec.TLS.SecretName
		if secretName == "" && spec.TLS.CertManager != nil {
			secretName = instance.Name + "-web-tls"
		}
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{spec.Host},
				SecretName: secretName,
			},
		}
	}

	return ingress
}
