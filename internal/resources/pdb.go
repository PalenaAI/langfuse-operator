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
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// BuildWebPDB constructs the desired PodDisruptionBudget for the Langfuse Web component.
func BuildWebPDB(instance *v1alpha1.LangfuseInstance) *policyv1.PodDisruptionBudget {
	return buildPDB(
		WebName(instance),
		instance.Namespace,
		CommonLabels(instance, "web"),
		SelectorLabels(instance, "web"),
		instance.Spec.Web.PodDisruptionBudget,
	)
}

// BuildWorkerPDB constructs the desired PodDisruptionBudget for the Langfuse Worker component.
func BuildWorkerPDB(instance *v1alpha1.LangfuseInstance) *policyv1.PodDisruptionBudget {
	return buildPDB(
		WorkerName(instance),
		instance.Namespace,
		CommonLabels(instance, "worker"),
		SelectorLabels(instance, "worker"),
		instance.Spec.Worker.PodDisruptionBudget,
	)
}

func buildPDB(name, namespace string, labels, selectorLabels map[string]string, spec *v1alpha1.PDBSpec) *policyv1.PodDisruptionBudget {
	minAvailable := intstr.FromInt32(1)
	if spec != nil && spec.MinAvailable != nil {
		minAvailable = intstr.FromInt32(*spec.MinAvailable)
	}

	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
		},
	}
}
