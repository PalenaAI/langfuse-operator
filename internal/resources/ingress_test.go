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

func TestBuildIngress_Basic(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Ingress = &v1alpha1.IngressSpec{
		Enabled:   true,
		ClassName: "nginx",
		Host:      testHostname,
	}

	ing := BuildIngress(instance)

	if ing.Name != testWebName {
		t.Errorf("name = %q, want %q", ing.Name, testWebName)
	}
	if ing.Namespace != instance.Namespace {
		t.Errorf("namespace = %q, want %q", ing.Namespace, instance.Namespace)
	}
	if ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName != "nginx" {
		t.Errorf("ingressClassName = %v, want %q", ing.Spec.IngressClassName, "nginx")
	}
	if len(ing.Spec.Rules) != 1 {
		t.Fatalf("rules count = %d, want 1", len(ing.Spec.Rules))
	}
	if ing.Spec.Rules[0].Host != testHostname {
		t.Errorf("host = %q, want %q", ing.Spec.Rules[0].Host, testHostname)
	}

	paths := ing.Spec.Rules[0].HTTP.Paths
	if len(paths) != 1 {
		t.Fatalf("paths count = %d, want 1", len(paths))
	}
	if paths[0].Backend.Service.Name != testWebName {
		t.Errorf("backend service = %q, want %q", paths[0].Backend.Service.Name, testWebName)
	}
	if paths[0].Backend.Service.Port.Number != 3000 {
		t.Errorf("backend port = %d, want 3000", paths[0].Backend.Service.Port.Number)
	}

	// No TLS
	if len(ing.Spec.TLS) != 0 {
		t.Errorf("TLS should be empty when not configured, got %d", len(ing.Spec.TLS))
	}
}

func TestBuildIngress_TLS(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Ingress = &v1alpha1.IngressSpec{
		Enabled: true,
		Host:    testHostname,
		TLS: &v1alpha1.IngressTLSSpec{
			Enabled:    true,
			SecretName: "my-tls-secret",
		},
	}

	ing := BuildIngress(instance)

	if len(ing.Spec.TLS) != 1 {
		t.Fatalf("TLS count = %d, want 1", len(ing.Spec.TLS))
	}
	if ing.Spec.TLS[0].SecretName != "my-tls-secret" {
		t.Errorf("TLS secret = %q, want %q", ing.Spec.TLS[0].SecretName, "my-tls-secret")
	}
	if len(ing.Spec.TLS[0].Hosts) != 1 || ing.Spec.TLS[0].Hosts[0] != testHostname {
		t.Errorf("TLS hosts = %v, want [langfuse.example.com]", ing.Spec.TLS[0].Hosts)
	}
}

func TestBuildIngress_CertManager(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Ingress = &v1alpha1.IngressSpec{
		Enabled: true,
		Host:    testHostname,
		TLS: &v1alpha1.IngressTLSSpec{
			Enabled: true,
			CertManager: &v1alpha1.CertManagerSpec{
				IssuerRef: v1alpha1.CertManagerIssuerRef{
					Name: "letsencrypt-prod",
					Kind: "ClusterIssuer",
				},
			},
		},
	}

	ing := BuildIngress(instance)

	if ing.Annotations["cert-manager.io/cluster-issuer"] != "letsencrypt-prod" {
		t.Errorf("cert-manager annotation = %q, want %q", ing.Annotations["cert-manager.io/cluster-issuer"], "letsencrypt-prod")
	}
	// Auto-generated TLS secret name
	if len(ing.Spec.TLS) != 1 || ing.Spec.TLS[0].SecretName != "test-web-tls" {
		t.Errorf("TLS secret = %q, want %q", ing.Spec.TLS[0].SecretName, "test-web-tls")
	}
}

func TestBuildIngress_Annotations(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Ingress = &v1alpha1.IngressSpec{
		Enabled: true,
		Host:    testHostname,
		Annotations: map[string]string{
			"nginx.ingress.kubernetes.io/proxy-body-size": "50m",
		},
	}

	ing := BuildIngress(instance)

	if ing.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"] != "50m" {
		t.Errorf("annotation missing or wrong")
	}
}
