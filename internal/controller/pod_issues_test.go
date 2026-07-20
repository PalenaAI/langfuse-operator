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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// waitingPod builds a pod whose single container is stuck in a waiting state.
func waitingPod(name, container, reason, message string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", Labels: labels},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  container,
				Ready: false,
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: reason, Message: message},
				},
			}},
		},
	}
}

func TestContainerIssue_Classification(t *testing.T) {
	cases := []struct {
		name       string
		status     corev1.ContainerStatus
		wantIssue  bool
		wantReason string
		wantFatal  bool
	}{
		{
			name:      "ready container is not an issue",
			status:    corev1.ContainerStatus{Name: "web", Ready: true},
			wantIssue: false,
		},
		{
			name: "ContainerCreating is a normal startup state",
			status: corev1.ContainerStatus{Name: "web", State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"},
			}},
			wantIssue: false,
		},
		{
			name: "PodInitializing is a normal startup state",
			status: corev1.ContainerStatus{Name: "web", State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"},
			}},
			wantIssue: false,
		},
		{
			name: "missing Secret key is fatal",
			status: corev1.ContainerStatus{Name: "web", State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{Reason: "CreateContainerConfigError"},
			}},
			wantIssue: true, wantReason: "CreateContainerConfigError", wantFatal: true,
		},
		{
			name: "bad image is fatal",
			status: corev1.ContainerStatus{Name: "web", State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"},
			}},
			wantIssue: true, wantReason: "ImagePullBackOff", wantFatal: true,
		},
		{
			name: "InvalidImageName is fatal",
			status: corev1.ContainerStatus{Name: "web", State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{Reason: "InvalidImageName"},
			}},
			wantIssue: true, wantReason: "InvalidImageName", wantFatal: true,
		},
		{
			// Langfuse containers legitimately crash-loop while waiting for
			// Postgres/ClickHouse on a cold start, so this must stay recoverable.
			name: "CrashLoopBackOff is reported but not fatal",
			status: corev1.ContainerStatus{Name: "web", RestartCount: 4, State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
			}},
			wantIssue: true, wantReason: "CrashLoopBackOff", wantFatal: false,
		},
		{
			name: "non-zero exit is reported",
			status: corev1.ContainerStatus{Name: "web", State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, Reason: "Error"},
			}},
			wantIssue: true, wantReason: "Error",
		},
		{
			name: "OOMKilled is reported",
			status: corev1.ContainerStatus{Name: "web", State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{ExitCode: 137, Reason: "OOMKilled"},
			}},
			wantIssue: true, wantReason: "OOMKilled",
		},
		{
			name: "clean exit is not an issue",
			status: corev1.ContainerStatus{Name: "migrate", State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
			}},
			wantIssue: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issue, ok := containerIssue("pod-1", tc.status)
			if ok != tc.wantIssue {
				t.Fatalf("reported issue = %v, want %v", ok, tc.wantIssue)
			}
			if !tc.wantIssue {
				return
			}
			if issue.Reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", issue.Reason, tc.wantReason)
			}
			if issue.Fatal != tc.wantFatal {
				t.Errorf("fatal = %v, want %v", issue.Fatal, tc.wantFatal)
			}
			if issue.Pod != "pod-1" {
				t.Errorf("pod = %q, want pod-1", issue.Pod)
			}
		})
	}
}

// A crash loop's waiting message says nothing useful ("back-off 5m0s
// restarting..."), so the previous termination must be folded in.
func TestWaitingMessage_IncludesPreviousTermination(t *testing.T) {
	cs := corev1.ContainerStatus{
		Name: "web",
		State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Reason:  "CrashLoopBackOff",
				Message: "back-off 5m0s restarting failed container",
			},
		},
		LastTerminationState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Message:  "ZodError: LANGFUSE_S3_EVENT_UPLOAD_BUCKET required",
			},
		},
	}

	msg := waitingMessage(cs)
	for _, want := range []string{"back-off", "exited with code 1", "ZodError"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
}

func TestPodIssues_Unschedulable(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "ns"},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Type:    corev1.PodScheduled,
				Status:  corev1.ConditionFalse,
				Reason:  "Unschedulable",
				Message: "0/3 nodes are available: insufficient cpu",
			}},
		},
	}

	issues := podIssues(pod)
	if len(issues) != 1 {
		t.Fatalf("issue count = %d, want 1", len(issues))
	}
	if issues[0].Reason != "Unschedulable" {
		t.Errorf("reason = %q, want Unschedulable", issues[0].Reason)
	}
	if issues[0].Fatal {
		t.Error("Unschedulable should not be fatal — cluster autoscaling may resolve it")
	}
}

// Terminating pods are unhealthy by definition; reporting them would emit noise
// on every rolling update.
func TestPodIssues_SkipsTerminatingAndSucceededPods(t *testing.T) {
	now := metav1.Now()
	terminating := waitingPod("web-1", "web", "CrashLoopBackOff", "", nil)
	terminating.DeletionTimestamp = &now
	terminating.Finalizers = []string{"keep/for-fake-client"}
	if issues := podIssues(terminating); len(issues) != 0 {
		t.Errorf("terminating pod produced %d issues, want 0", len(issues))
	}

	succeeded := waitingPod("migrate-1", "migrate", "CrashLoopBackOff", "", nil)
	succeeded.Status.Phase = corev1.PodSucceeded
	if issues := podIssues(succeeded); len(issues) != 0 {
		t.Errorf("succeeded pod produced %d issues, want 0", len(issues))
	}
}

func TestCollectPodIssues_FiltersBySelectorAndSortsFatalFirst(t *testing.T) {
	webLabels := map[string]string{
		"app.kubernetes.io/name":      "langfuse",
		"app.kubernetes.io/instance":  "test",
		"app.kubernetes.io/component": "web",
	}
	workerLabels := map[string]string{
		"app.kubernetes.io/name":      "langfuse",
		"app.kubernetes.io/instance":  "test",
		"app.kubernetes.io/component": "worker",
	}

	c := newFakeClient(t,
		waitingPod("web-b", "langfuse-web", "CrashLoopBackOff", "", webLabels),
		waitingPod("web-a", "langfuse-web", "ImagePullBackOff", "bad image", webLabels),
		waitingPod("worker-a", "langfuse-worker", "CrashLoopBackOff", "", workerLabels),
	)

	issues, err := collectPodIssues(context.Background(), c, "ns", webLabels)
	if err != nil {
		t.Fatalf("collectPodIssues() error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("issue count = %d, want 2 (worker pod must be filtered out)", len(issues))
	}
	// Fatal issues sort first so the condition reason names the actionable one.
	if issues[0].Reason != "ImagePullBackOff" || !issues[0].Fatal {
		t.Errorf("first issue = %+v, want the fatal ImagePullBackOff", issues[0])
	}
	if !hasFatalIssue(issues) {
		t.Error("hasFatalIssue() = false, want true")
	}
}

func TestCollectPodIssues_HealthyPodsProduceNothing(t *testing.T) {
	labels := map[string]string{
		"app.kubernetes.io/name":      "langfuse",
		"app.kubernetes.io/instance":  "test",
		"app.kubernetes.io/component": "web",
	}
	healthy := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "ns", Labels: labels},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Name: "langfuse-web", Ready: true}},
		},
	}

	issues, err := collectPodIssues(context.Background(), newFakeClient(t, healthy), "ns", labels)
	if err != nil {
		t.Fatalf("collectPodIssues() error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("healthy pod produced %d issues, want 0: %+v", len(issues), issues)
	}
}

func TestSummarizePodIssues(t *testing.T) {
	t.Run("falls back when there are no issues", func(t *testing.T) {
		reason, msg := summarizePodIssues(nil, "DeploymentNotReady", "web has 0/1 ready replicas")
		if reason != "DeploymentNotReady" {
			t.Errorf("reason = %q, want DeploymentNotReady", reason)
		}
		if msg != "web has 0/1 ready replicas" {
			t.Errorf("message = %q, want the replica summary", msg)
		}
	})

	t.Run("uses the primary issue reason and details every pod", func(t *testing.T) {
		issues := []v1alpha1.PodIssue{
			{Pod: "web-a", Container: "langfuse-web", Reason: "CreateContainerConfigError",
				Message: `secret "gen" key "admin-api-key" not found`, Fatal: true},
			{Pod: "web-b", Container: "langfuse-web", Reason: "CrashLoopBackOff"},
		}
		reason, msg := summarizePodIssues(issues, "DeploymentNotReady", "web has 0/2 ready replicas")

		if reason != "CreateContainerConfigError" {
			t.Errorf("reason = %q, want the primary issue reason", reason)
		}
		for _, want := range []string{"web has 0/2 ready replicas", "web-a", "admin-api-key", "web-b", "CrashLoopBackOff"} {
			if !strings.Contains(msg, want) {
				t.Errorf("message %q missing %q", msg, want)
			}
		}
	})
}

// Status is not a log: a widely-broken Deployment must not push an unbounded
// list into the CR.
func TestCollectPodIssues_CapsReportedIssues(t *testing.T) {
	labels := map[string]string{
		"app.kubernetes.io/name":      "langfuse",
		"app.kubernetes.io/instance":  "test",
		"app.kubernetes.io/component": "web",
	}
	names := []string{"a", "b", "c", "d", "e", "f", "g"}
	objs := make([]client.Object, 0, len(names))
	for _, n := range names {
		objs = append(objs, waitingPod("web-"+n, "langfuse-web", "CrashLoopBackOff", "", labels))
	}

	issues, err := collectPodIssues(context.Background(), newFakeClient(t, objs...), "ns", labels)
	if err != nil {
		t.Fatalf("collectPodIssues() error: %v", err)
	}
	if len(issues) != maxReportedPodIssues {
		t.Errorf("issue count = %d, want cap of %d", len(issues), maxReportedPodIssues)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short) = %q, want unchanged", got)
	}
	got := truncate(strings.Repeat("x", 20), 10)
	if len([]rune(got)) != 11 { // 10 chars + the ellipsis rune
		t.Errorf("truncate produced %q (%d runes), want 10 chars + ellipsis", got, len([]rune(got)))
	}
}
