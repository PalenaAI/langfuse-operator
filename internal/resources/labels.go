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

import v1alpha1 "github.com/bitkaio/langfuse-operator/api/v1alpha1"

// CommonLabels returns labels that should be applied to all managed resources.
func CommonLabels(instance *v1alpha1.LangfuseInstance, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "langfuse",
		"app.kubernetes.io/instance":   instance.Name,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": "langfuse-operator",
		"app.kubernetes.io/part-of":    "langfuse",
		"langfuse.palena.ai/instance":  instance.Name,
	}
}

// SelectorLabels returns the minimal set of labels used for pod selectors.
func SelectorLabels(instance *v1alpha1.LangfuseInstance, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "langfuse",
		"app.kubernetes.io/instance":  instance.Name,
		"app.kubernetes.io/component": component,
	}
}

// WebName returns the name for Web resources.
func WebName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-web"
}

// WorkerName returns the name for Worker resources.
func WorkerName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-worker"
}

// WebServiceName returns the name for the Web Service.
func WebServiceName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-web"
}
