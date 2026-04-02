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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// SecretKeyRef references a key within a Kubernetes Secret.
type SecretKeyRef struct {
	// Name is the name of the Secret.
	Name string `json:"name"`
	// Key is the key within the Secret.
	Key string `json:"key"`
}

// SecretKeysRef references multiple keys within a single Secret.
type SecretKeysRef struct {
	// Name is the name of the Secret.
	Name string `json:"name"`
	// Keys maps logical names to Secret keys.
	Keys map[string]string `json:"keys"`
}

// ResourceRequirements defines compute resource requirements.
type ResourceRequirements struct {
	// Requests describes the minimum resources required.
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
	// Limits describes the maximum resources allowed.
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
}

// ObjectReference is a reference to another CR in the same or different namespace.
type ObjectReference struct {
	// Name of the referenced resource.
	Name string `json:"name"`
	// Namespace of the referenced resource. Defaults to the referencing resource's namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}
