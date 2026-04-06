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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// ServiceMonitorName returns the name for the Prometheus ServiceMonitor resource.
func ServiceMonitorName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-web"
}

// BuildServiceMonitor constructs a Prometheus ServiceMonitor as an unstructured object.
// This avoids importing the prometheus-operator API types as a dependency.
func BuildServiceMonitor(instance *v1alpha1.LangfuseInstance) *unstructured.Unstructured {
	labels := CommonLabels(instance, "web")

	// Merge additional labels from the ServiceMonitor spec.
	if instance.Spec.Observability != nil &&
		instance.Spec.Observability.ServiceMonitor != nil &&
		instance.Spec.Observability.ServiceMonitor.Labels != nil {
		for k, v := range instance.Spec.Observability.ServiceMonitor.Labels {
			labels[k] = v
		}
	}

	// Determine scrape interval, defaulting to "30s".
	interval := "30s"
	if instance.Spec.Observability != nil &&
		instance.Spec.Observability.ServiceMonitor != nil &&
		instance.Spec.Observability.ServiceMonitor.Interval != "" {
		interval = instance.Spec.Observability.ServiceMonitor.Interval
	}

	// Selector labels match the web component pods.
	selectorLabels := map[string]interface{}{
		"app.kubernetes.io/name":      "langfuse",
		"app.kubernetes.io/instance":  instance.Name,
		"app.kubernetes.io/component": "web",
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "ServiceMonitor",
			"metadata": map[string]interface{}{
				"name":      ServiceMonitorName(instance),
				"namespace": instance.Namespace,
				"labels":    toStringInterfaceMap(labels),
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": selectorLabels,
				},
				"endpoints": []interface{}{
					map[string]interface{}{
						"port":     "http",
						"path":     "/api/public/health",
						"interval": interval,
					},
				},
			},
		},
	}
}
