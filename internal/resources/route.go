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

// RouteName returns the name for the OpenShift Route resource.
func RouteName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-web"
}

// BuildRoute constructs an OpenShift Route as an unstructured object.
// This avoids importing the OpenShift API types as a dependency.
func BuildRoute(instance *v1alpha1.LangfuseInstance) *unstructured.Unstructured {
	spec := instance.Spec.Route
	labels := CommonLabels(instance, "web")

	annotations := make(map[string]string)
	for k, v := range spec.Annotations {
		annotations[k] = v
	}

	route := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "route.openshift.io/v1",
			"kind":       "Route",
			"metadata": map[string]interface{}{
				"name":        RouteName(instance),
				"namespace":   instance.Namespace,
				"labels":      toStringInterfaceMap(labels),
				"annotations": toStringInterfaceMap(annotations),
			},
			"spec": map[string]interface{}{
				"to": map[string]interface{}{
					"kind":   "Service",
					"name":   WebServiceName(instance),
					"weight": int64(100),
				},
				"port": map[string]interface{}{
					"targetPort": "http",
				},
				"tls": map[string]interface{}{
					"termination":                   "edge",
					"insecureEdgeTerminationPolicy": "Redirect",
				},
			},
		},
	}

	if spec.Host != "" {
		routeSpec := route.Object["spec"].(map[string]interface{})
		routeSpec["host"] = spec.Host
	}

	return route
}

func toStringInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
