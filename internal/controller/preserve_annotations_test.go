/*
Copyright 2026 bitkaio LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func deploymentWithPodAnnotations(a map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Annotations: a},
			},
		},
	}
}

func TestPreservePodTemplateAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		existing map[string]string
		desired  map[string]string
		want     map[string]string
	}{
		{
			name:     "copies operator-namespaced annotation when desired is empty",
			existing: map[string]string{"langfuse.palena.ai/secret-hash": "abc"},
			desired:  nil,
			want:     map[string]string{"langfuse.palena.ai/secret-hash": "abc"},
		},
		{
			name:     "does not overwrite when desired already sets the same key",
			existing: map[string]string{"langfuse.palena.ai/secret-hash": "OLD"},
			desired:  map[string]string{"langfuse.palena.ai/secret-hash": "NEW"},
			want:     map[string]string{"langfuse.palena.ai/secret-hash": "NEW"},
		},
		{
			name:     "ignores annotations outside the operator namespace",
			existing: map[string]string{"some.other/key": "v"},
			desired:  nil,
			want:     nil,
		},
		{
			name: "preserves only operator-namespaced keys, drops the rest",
			existing: map[string]string{
				"langfuse.palena.ai/secret-hash": "abc",
				"unrelated.example.com/key":      "v",
			},
			desired: map[string]string{"langfuse.palena.ai/other": "z"},
			want: map[string]string{
				"langfuse.palena.ai/secret-hash": "abc",
				"langfuse.palena.ai/other":       "z",
			},
		},
		{
			name:     "no-op when existing has no annotations",
			existing: nil,
			desired:  nil,
			want:     nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			existing := deploymentWithPodAnnotations(tc.existing)
			desired := deploymentWithPodAnnotations(tc.desired)

			preservePodTemplateAnnotations(existing, desired)

			got := desired.Spec.Template.Annotations
			if !annotationsEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func annotationsEqual(a, b map[string]string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
