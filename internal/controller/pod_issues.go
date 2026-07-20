/*
Copyright 2026 bitkaio LLC.

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

package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// Component names, matching the app.kubernetes.io/component label the resource
// builders stamp onto each workload.
const (
	componentWeb       = "web"
	componentWorker    = "worker"
	componentMigration = "migration"
)

// maxReportedPodIssues caps how many pod issues are written into status. A
// broken Deployment can produce one issue per replica, and status is not a log
// — a handful is enough to diagnose, and it keeps the CR well clear of the
// etcd object size limit.
const maxReportedPodIssues = 5

// fatalWaitingReasons are container waiting reasons that cannot resolve without
// a human changing the spec or a referenced object: a bad image reference, or a
// Secret/ConfigMap key that does not exist. Everything else (notably
// CrashLoopBackOff) is treated as potentially transient, because Langfuse
// containers legitimately crash-loop while waiting for Postgres or ClickHouse
// to accept connections during a cold start.
var fatalWaitingReasons = map[string]bool{
	"ImagePullBackOff":           true,
	"ErrImagePull":               true,
	"InvalidImageName":           true,
	"ErrImageNeverPull":          true,
	"CreateContainerConfigError": true,
	"CreateContainerError":       true,
}

// benignWaitingReasons are the normal states of a pod that is still starting.
// They are not reported as issues.
var benignWaitingReasons = map[string]bool{
	"":                  true,
	"ContainerCreating": true,
	"PodInitializing":   true,
}

// collectPodIssues lists the pods matching selector and returns the pod-level
// problems preventing them from running. It returns an empty slice when every
// pod is healthy or still starting normally.
func collectPodIssues(ctx context.Context, c client.Client, namespace string, selector map[string]string) ([]v1alpha1.PodIssue, error) {
	podList := &corev1.PodList{}
	if err := c.List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	); err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var issues []v1alpha1.PodIssue
	for i := range podList.Items {
		issues = append(issues, podIssues(&podList.Items[i])...)
	}

	// Deterministic ordering keeps status writes stable across reconciles, so
	// we don't churn resourceVersion (and re-trigger watchers) on every pass.
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Fatal != issues[j].Fatal {
			return issues[i].Fatal // fatal issues first — they're the actionable ones
		}
		if issues[i].Pod != issues[j].Pod {
			return issues[i].Pod < issues[j].Pod
		}
		return issues[i].Container < issues[j].Container
	})

	if len(issues) > maxReportedPodIssues {
		issues = issues[:maxReportedPodIssues]
	}
	return issues, nil
}

// podIssues extracts the problems from a single pod's status.
func podIssues(pod *corev1.Pod) []v1alpha1.PodIssue {
	// A terminating pod is expected to be unhealthy; reporting it would surface
	// noise during every rolling update.
	if pod.DeletionTimestamp != nil {
		return nil
	}
	if pod.Status.Phase == corev1.PodSucceeded {
		return nil
	}

	var issues []v1alpha1.PodIssue

	// Pod-level: unschedulable (insufficient resources, unsatisfiable affinity
	// or topology constraints, missing PVC).
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Reason != "" {
			issues = append(issues, v1alpha1.PodIssue{
				Pod:     pod.Name,
				Reason:  cond.Reason,
				Message: cond.Message,
			})
		}
	}

	// Container-level. Init containers are checked first: the migration Job's
	// wait-for-stores init container failing is the most informative signal
	// about why migrations are stuck.
	statuses := make([]corev1.ContainerStatus, 0,
		len(pod.Status.InitContainerStatuses)+len(pod.Status.ContainerStatuses))
	statuses = append(statuses, pod.Status.InitContainerStatuses...)
	statuses = append(statuses, pod.Status.ContainerStatuses...)

	for _, cs := range statuses {
		if issue, ok := containerIssue(pod.Name, cs); ok {
			issues = append(issues, issue)
		}
	}

	return issues
}

// containerIssue maps a single container status to a PodIssue, reporting
// whether the container is actually in a problem state.
func containerIssue(podName string, cs corev1.ContainerStatus) (v1alpha1.PodIssue, bool) {
	if cs.Ready {
		return v1alpha1.PodIssue{}, false
	}

	switch {
	case cs.State.Waiting != nil:
		reason := cs.State.Waiting.Reason
		if benignWaitingReasons[reason] {
			return v1alpha1.PodIssue{}, false
		}
		return v1alpha1.PodIssue{
			Pod:          podName,
			Container:    cs.Name,
			Reason:       reason,
			Message:      waitingMessage(cs),
			RestartCount: cs.RestartCount,
			Fatal:        fatalWaitingReasons[reason],
		}, true

	case cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0:
		term := cs.State.Terminated
		reason := term.Reason
		if reason == "" {
			reason = "Error"
		}
		return v1alpha1.PodIssue{
			Pod:          podName,
			Container:    cs.Name,
			Reason:       reason,
			Message:      terminationMessage(term),
			RestartCount: cs.RestartCount,
		}, true
	}

	return v1alpha1.PodIssue{}, false
}

// waitingMessage builds the detail for a waiting container. For a crash loop
// the waiting message alone ("back-off 5m0s restarting failed container") says
// nothing about the cause, so the previous termination is appended — that's
// where the exit code and any captured output live.
func waitingMessage(cs corev1.ContainerStatus) string {
	msg := cs.State.Waiting.Message
	if cs.LastTerminationState.Terminated != nil {
		if last := terminationMessage(cs.LastTerminationState.Terminated); last != "" {
			if msg == "" {
				return "previous run " + last
			}
			return msg + "; previous run " + last
		}
	}
	return msg
}

// terminationMessage summarises a container termination, preferring the
// container's own terminationMessage (Kubernetes captures the tail of the log
// for crashed containers) over a bare exit code.
func terminationMessage(term *corev1.ContainerStateTerminated) string {
	parts := []string{fmt.Sprintf("exited with code %d", term.ExitCode)}
	if term.Reason != "" && term.Reason != "Error" {
		parts = append(parts, "("+term.Reason+")")
	}
	if detail := strings.TrimSpace(term.Message); detail != "" {
		parts = append(parts, "- "+truncate(detail, 256))
	}
	return strings.Join(parts, " ")
}

// hasFatalIssue reports whether any issue requires human intervention.
func hasFatalIssue(issues []v1alpha1.PodIssue) bool {
	for _, issue := range issues {
		if issue.Fatal {
			return true
		}
	}
	return false
}

// summarizePodIssues renders issues into a condition reason and message. The
// reason is taken from the most significant issue (fatal first, per the sort in
// collectPodIssues) so it stays a valid single-token condition reason.
func summarizePodIssues(issues []v1alpha1.PodIssue, fallbackReason, fallbackMessage string) (reason, message string) {
	if len(issues) == 0 {
		return fallbackReason, fallbackMessage
	}

	primary := issues[0]
	descriptions := make([]string, 0, len(issues))
	for _, issue := range issues {
		desc := issue.Pod
		if issue.Container != "" {
			desc += " (" + issue.Container + ")"
		}
		desc += ": " + issue.Reason
		if issue.Message != "" {
			desc += " — " + issue.Message
		}
		descriptions = append(descriptions, desc)
	}

	return primary.Reason, truncate(
		fmt.Sprintf("%s; %s", fallbackMessage, strings.Join(descriptions, "; ")), 1024)
}

// truncate shortens s to at most max characters. Condition messages are capped
// at 32KiB by the API server, and an unbounded container log tail could blow
// past that.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
