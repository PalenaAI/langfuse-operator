/*
Copyright 2026 bitkaio LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// newFakeClientWithApps builds a fake client that also knows about Deployments.
func newFakeClientWithApps(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, appsv1.AddToScheme, v1alpha1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatalf("scheme: %v", err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func podTestInstance() *v1alpha1.LangfuseInstance {
	return &v1alpha1.LangfuseInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
		Spec: v1alpha1.LangfuseInstanceSpec{
			Image: v1alpha1.ImageSpec{Tag: "3"},
			Auth:  v1alpha1.AuthSpec{NextAuthUrl: "https://langfuse.example.com"},
		},
	}
}

func notReadyDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	}
}

// The headline behaviour: a pod that cannot start must explain itself in the
// component condition instead of only reporting a replica count.
func TestCheckComponentDeployment_SurfacesPodFailure(t *testing.T) {
	instance := podTestInstance()
	labels := map[string]string{
		"app.kubernetes.io/name":      "langfuse",
		"app.kubernetes.io/instance":  "test",
		"app.kubernetes.io/component": "web",
	}
	brokenPod := waitingPod("test-web-abc12", "langfuse-web",
		"CreateContainerConfigError", `secret "test-generated-secrets" key "admin-api-key" not found`, labels)

	r := &HealthMonitorReconciler{Client: newFakeClientWithApps(t, notReadyDeployment("test-web"), brokenPod)}

	condition, issues := r.checkComponentDeployment(context.Background(), instance,
		componentWeb, "test-web", conditionWebReady)

	if condition.Status != metav1.ConditionFalse {
		t.Errorf("status = %v, want False", condition.Status)
	}
	if condition.Reason != "CreateContainerConfigError" {
		t.Errorf("reason = %q, want CreateContainerConfigError (not a generic DeploymentNotReady)", condition.Reason)
	}
	if !strings.Contains(condition.Message, "admin-api-key") {
		t.Errorf("message %q should carry the underlying Kubernetes detail", condition.Message)
	}
	if len(issues) != 1 || !issues[0].Fatal {
		t.Fatalf("issues = %+v, want one fatal issue", issues)
	}
}

func TestCheckComponentDeployment_ReadyReportsNoIssues(t *testing.T) {
	instance := podTestInstance()
	ready := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-web", Namespace: "ns"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	}
	r := &HealthMonitorReconciler{Client: newFakeClientWithApps(t, ready)}

	condition, issues := r.checkComponentDeployment(context.Background(), instance,
		componentWeb, "test-web", conditionWebReady)

	if condition.Status != metav1.ConditionTrue {
		t.Errorf("status = %v, want True", condition.Status)
	}
	if len(issues) != 0 {
		t.Errorf("issues = %+v, want none for a ready deployment", issues)
	}
}

func TestCheckWebAndWorker_PopulateStatusIssues(t *testing.T) {
	instance := podTestInstance()
	webLabels := map[string]string{
		"app.kubernetes.io/name": "langfuse", "app.kubernetes.io/instance": "test",
		"app.kubernetes.io/component": "web",
	}
	workerLabels := map[string]string{
		"app.kubernetes.io/name": "langfuse", "app.kubernetes.io/instance": "test",
		"app.kubernetes.io/component": "worker",
	}

	r := &HealthMonitorReconciler{Client: newFakeClientWithApps(t,
		notReadyDeployment("test-web"),
		notReadyDeployment("test-worker"),
		waitingPod("test-web-1", "langfuse-web", "ImagePullBackOff", "bad tag", webLabels),
		waitingPod("test-worker-1", "langfuse-worker", "CrashLoopBackOff", "", workerLabels),
	)}

	r.checkWebDeployment(context.Background(), instance)
	r.checkWorkerDeployment(context.Background(), instance)

	// Both components must report — the Worker is where ingestion happens, so a
	// Worker-only failure must not be invisible.
	if instance.Status.Web == nil || len(instance.Status.Web.Issues) != 1 {
		t.Fatalf("web issues = %+v, want 1", instance.Status.Web)
	}
	if instance.Status.Worker == nil || len(instance.Status.Worker.Issues) != 1 {
		t.Fatalf("worker issues = %+v, want 1", instance.Status.Worker)
	}
	if instance.Status.Web.Issues[0].Reason != "ImagePullBackOff" {
		t.Errorf("web reason = %q", instance.Status.Web.Issues[0].Reason)
	}
	if instance.Status.Worker.Issues[0].Reason != "CrashLoopBackOff" {
		t.Errorf("worker reason = %q", instance.Status.Worker.Issues[0].Reason)
	}
}

func TestDetermineOverallHealth_Phases(t *testing.T) {
	allTrue := []metav1.Condition{{Status: metav1.ConditionTrue}, {Status: metav1.ConditionTrue}}
	someFalse := []metav1.Condition{{Status: metav1.ConditionTrue}, {Status: metav1.ConditionFalse}}

	cases := []struct {
		name       string
		conditions []metav1.Condition
		issues     []v1alpha1.PodIssue
		wantPhase  string
		wantReady  bool
	}{
		{"all healthy", allTrue, nil, phaseRunning, true},
		{"recoverable failure is Degraded", someFalse,
			[]v1alpha1.PodIssue{{Reason: "CrashLoopBackOff"}}, phaseDegraded, false},
		{"no pod detail is Degraded", someFalse, nil, phaseDegraded, false},
		{"fatal misconfiguration is Error", someFalse,
			[]v1alpha1.PodIssue{{Reason: "ImagePullBackOff", Fatal: true}}, phaseError, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			instance := podTestInstance()
			instance.Status.Web = &v1alpha1.ComponentStatus{Issues: tc.issues}

			r := &HealthMonitorReconciler{}
			r.determineOverallHealth(instance, tc.conditions)

			if instance.Status.Phase != tc.wantPhase {
				t.Errorf("phase = %q, want %q", instance.Status.Phase, tc.wantPhase)
			}
			if instance.Status.Ready != tc.wantReady {
				t.Errorf("ready = %v, want %v", instance.Status.Ready, tc.wantReady)
			}
		})
	}
}

// A fatal issue on the Worker alone must still drive the instance to Error.
func TestInstanceHasFatalPodIssue_ChecksBothComponents(t *testing.T) {
	instance := podTestInstance()
	instance.Status.Worker = &v1alpha1.WorkerComponentStatus{
		ComponentStatus: v1alpha1.ComponentStatus{
			Issues: []v1alpha1.PodIssue{{Reason: "CreateContainerConfigError", Fatal: true}},
		},
	}
	if !instanceHasFatalPodIssue(instance) {
		t.Error("a fatal Worker issue should mark the instance fatal")
	}
}
