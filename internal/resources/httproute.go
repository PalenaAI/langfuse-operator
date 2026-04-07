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

// HTTPRouteName returns the name for the Gateway API HTTPRoute resource.
func HTTPRouteName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-web"
}

// BuildHTTPRoute constructs a Gateway API HTTPRoute as an unstructured object.
// This avoids importing the Gateway API types as a dependency.
func BuildHTTPRoute(instance *v1alpha1.LangfuseInstance) *unstructured.Unstructured {
	spec := instance.Spec.GatewayAPI
	labels := CommonLabels(instance, "web")

	annotations := make(map[string]string)
	for k, v := range spec.Annotations {
		annotations[k] = v
	}

	parentRef := map[string]interface{}{
		"name": spec.GatewayRef.Name,
	}
	if spec.GatewayRef.Namespace != "" {
		parentRef["namespace"] = spec.GatewayRef.Namespace
	}
	if spec.GatewayRef.SectionName != "" {
		parentRef["sectionName"] = spec.GatewayRef.SectionName
	}

	routeSpec := map[string]interface{}{
		"parentRefs": []interface{}{parentRef},
		"rules": []interface{}{
			map[string]interface{}{
				"matches": []interface{}{
					map[string]interface{}{
						"path": map[string]interface{}{
							"type":  "PathPrefix",
							"value": "/",
						},
					},
				},
				"backendRefs": []interface{}{
					map[string]interface{}{
						"name": WebServiceName(instance),
						"port": int64(3000),
					},
				},
			},
		},
	}

	if spec.Hostname != "" {
		routeSpec["hostnames"] = []interface{}{spec.Hostname}
	}

	httpRoute := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "HTTPRoute",
			"metadata": map[string]interface{}{
				"name":        HTTPRouteName(instance),
				"namespace":   instance.Namespace,
				"labels":      toStringInterfaceMap(labels),
				"annotations": toStringInterfaceMap(annotations),
			},
			"spec": routeSpec,
		},
	}

	return httpRoute
}
