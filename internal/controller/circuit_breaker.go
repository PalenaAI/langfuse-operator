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

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
	"github.com/PalenaAI/langfuse-operator/internal/resources"
)

const (
	defaultProbeIntervalSeconds = 15
	defaultFailureThreshold     = 3
)

// CircuitBreakerController monitors dependency health and takes protective
// actions (e.g., scaling worker to zero) when dependencies are unavailable.
type CircuitBreakerController struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// In-memory state (resets on operator restart).
	// Maps are keyed by instance namespace/name, then by component name.
	failureCounts map[string]map[string]int
	savedReplicas map[string]map[string]int32
}

// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *CircuitBreakerController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the LangfuseInstance
	instance := &v1alpha1.LangfuseInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			// Clean up in-memory state for deleted instances
			instanceKey := req.String()
			delete(r.failureCounts, instanceKey)
			delete(r.savedReplicas, instanceKey)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching LangfuseInstance: %w", err)
	}

	// 2. If circuit breaker is disabled, return
	if instance.Spec.CircuitBreaker == nil {
		return ctrl.Result{}, nil
	}
	if instance.Spec.CircuitBreaker.Enabled != nil && !*instance.Spec.CircuitBreaker.Enabled {
		return ctrl.Result{}, nil
	}

	// 3. Initialize in-memory maps if nil
	if r.failureCounts == nil {
		r.failureCounts = make(map[string]map[string]int)
	}
	if r.savedReplicas == nil {
		r.savedReplicas = make(map[string]map[string]int32)
	}

	instanceKey := req.String()
	if r.failureCounts[instanceKey] == nil {
		r.failureCounts[instanceKey] = make(map[string]int)
	}
	if r.savedReplicas[instanceKey] == nil {
		r.savedReplicas[instanceKey] = make(map[string]int32)
	}

	cb := instance.Spec.CircuitBreaker
	shortestInterval := time.Duration(defaultProbeIntervalSeconds) * time.Second

	// 4. Check each component
	type componentCheck struct {
		name string
		spec *v1alpha1.ComponentCircuitBreakerSpec
	}
	components := []componentCheck{
		{name: "clickhouse", spec: cb.ClickHouse},
		{name: "redis", spec: cb.Redis},
		{name: "database", spec: cb.Database},
	}

	statusChanged := false
	for _, comp := range components {
		if comp.spec == nil {
			continue
		}

		probeInterval := time.Duration(comp.spec.ProbeIntervalSeconds) * time.Second
		if comp.spec.ProbeIntervalSeconds <= 0 {
			probeInterval = time.Duration(defaultProbeIntervalSeconds) * time.Second
		}
		if probeInterval < shortestInterval {
			shortestInterval = probeInterval
		}

		threshold := int(comp.spec.FailureThreshold)
		if threshold <= 0 {
			threshold = defaultFailureThreshold
		}

		// TODO: Perform actual health probe against the component.
		// For now, assume success (healthy). When the connection infrastructure
		// is implemented, this will make HTTP/TCP probes against each dependency.
		probeSuccess := true
		log.V(1).Info("circuit breaker probe",
			"component", comp.name,
			"healthy", probeSuccess,
			"failureCount", r.failureCounts[instanceKey][comp.name],
			"threshold", threshold,
		)

		if probeSuccess {
			// Reset failure counter
			previousFailures := r.failureCounts[instanceKey][comp.name]
			r.failureCounts[instanceKey][comp.name] = 0

			// Check if we're recovering from an open circuit
			if previousFailures >= threshold {
				changed, err := r.recoverCircuitBreaker(ctx, instance, instanceKey, comp.name, comp.spec)
				if err != nil {
					log.Error(err, "failed to recover circuit breaker", "component", comp.name)
				}
				if changed {
					statusChanged = true
				}
			}
		} else {
			// Increment failure counter
			r.failureCounts[instanceKey][comp.name]++
			count := r.failureCounts[instanceKey][comp.name]

			if count >= threshold {
				changed, err := r.openCircuitBreaker(ctx, instance, instanceKey, comp.name, comp.spec)
				if err != nil {
					log.Error(err, "failed to open circuit breaker", "component", comp.name)
				}
				if changed {
					statusChanged = true
				}
			}
		}
	}

	// 5. Update status conditions
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "CircuitBreakerHealthy",
		Status:             boolToConditionStatus(!isCircuitBreakerActive(instance)),
		Reason:             circuitBreakerReason(instance),
		Message:            circuitBreakerMessage(instance),
		ObservedGeneration: instance.Generation,
	})

	if statusChanged {
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating circuit breaker status: %w", err)
		}
	} else {
		// Still update conditions even if no circuit state changed
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating circuit breaker status: %w", err)
		}
	}

	return ctrl.Result{RequeueAfter: shortestInterval}, nil
}

// openCircuitBreaker takes the configured action when the failure threshold is reached.
func (r *CircuitBreakerController) openCircuitBreaker(ctx context.Context, instance *v1alpha1.LangfuseInstance, instanceKey, component string, spec *v1alpha1.ComponentCircuitBreakerSpec) (bool, error) {
	log := logf.FromContext(ctx)

	if spec.Action != "scaleWorkerToZero" {
		if spec.Action == "emitCriticalEvent" {
			r.Recorder.Eventf(instance, "Warning", "CircuitBreakerOpen",
				"Circuit breaker opened for %s: dependency unhealthy", component)
		}
		return false, nil
	}

	// Check if already tripped
	if instance.Status.Worker != nil && instance.Status.Worker.CircuitBreakerActive {
		return false, nil
	}

	// Save current worker replica count
	workerDeploy := &appsv1.Deployment{}
	workerKey := types.NamespacedName{Name: resources.WorkerName(instance), Namespace: instance.Namespace}
	if err := r.Get(ctx, workerKey, workerDeploy); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting worker deployment: %w", err)
	}

	currentReplicas := int32(1)
	if workerDeploy.Spec.Replicas != nil {
		currentReplicas = *workerDeploy.Spec.Replicas
	}
	r.savedReplicas[instanceKey][component] = currentReplicas

	// Scale worker to 0
	zero := int32(0)
	patch := client.MergeFrom(workerDeploy.DeepCopy())
	workerDeploy.Spec.Replicas = &zero
	if err := r.Patch(ctx, workerDeploy, patch); err != nil {
		return false, fmt.Errorf("scaling worker to zero: %w", err)
	}

	log.Info("circuit breaker opened: scaled worker to 0",
		"component", component,
		"savedReplicas", currentReplicas,
	)

	// Update worker status
	if instance.Status.Worker == nil {
		instance.Status.Worker = &v1alpha1.WorkerComponentStatus{}
	}
	instance.Status.Worker.CircuitBreakerActive = true
	instance.Status.Worker.CircuitBreakerReason = fmt.Sprintf("%s dependency unhealthy", component)

	r.Recorder.Eventf(instance, "Warning", "CircuitBreakerOpen",
		"Scaled worker to 0: %s dependency unhealthy (failures=%d)",
		component, r.failureCounts[instanceKey][component])

	return true, nil
}

// recoverCircuitBreaker restores the component after recovery.
func (r *CircuitBreakerController) recoverCircuitBreaker(ctx context.Context, instance *v1alpha1.LangfuseInstance, instanceKey, component string, spec *v1alpha1.ComponentCircuitBreakerSpec) (bool, error) {
	log := logf.FromContext(ctx)

	if spec.Action != "scaleWorkerToZero" || spec.RecoveryAction != "restoreScale" {
		if spec.Action == "emitCriticalEvent" {
			r.Recorder.Eventf(instance, "Normal", "CircuitBreakerClosed",
				"Circuit breaker closed for %s: dependency recovered", component)
		}
		return false, nil
	}

	// Restore saved replica count
	savedReplicas, ok := r.savedReplicas[instanceKey][component]
	if !ok || savedReplicas == 0 {
		savedReplicas = 1 // Default to 1 if no saved state
	}

	workerDeploy := &appsv1.Deployment{}
	workerKey := types.NamespacedName{Name: resources.WorkerName(instance), Namespace: instance.Namespace}
	if err := r.Get(ctx, workerKey, workerDeploy); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting worker deployment for recovery: %w", err)
	}

	patch := client.MergeFrom(workerDeploy.DeepCopy())
	workerDeploy.Spec.Replicas = &savedReplicas
	if err := r.Patch(ctx, workerDeploy, patch); err != nil {
		return false, fmt.Errorf("restoring worker replicas: %w", err)
	}

	log.Info("circuit breaker closed: restored worker replicas",
		"component", component,
		"replicas", savedReplicas,
	)

	// Update worker status
	if instance.Status.Worker == nil {
		instance.Status.Worker = &v1alpha1.WorkerComponentStatus{}
	}
	instance.Status.Worker.CircuitBreakerActive = false
	instance.Status.Worker.CircuitBreakerReason = ""

	// Clean up saved state
	delete(r.savedReplicas[instanceKey], component)

	r.Recorder.Eventf(instance, "Normal", "CircuitBreakerClosed",
		"Restored worker to %d replicas: %s dependency recovered",
		savedReplicas, component)

	return true, nil
}

func isCircuitBreakerActive(instance *v1alpha1.LangfuseInstance) bool {
	return instance.Status.Worker != nil && instance.Status.Worker.CircuitBreakerActive
}

func circuitBreakerReason(instance *v1alpha1.LangfuseInstance) string {
	if isCircuitBreakerActive(instance) {
		return "CircuitOpen"
	}
	return "AllDependenciesHealthy"
}

func circuitBreakerMessage(instance *v1alpha1.LangfuseInstance) string {
	if instance.Status.Worker != nil && instance.Status.Worker.CircuitBreakerActive {
		return fmt.Sprintf("Circuit breaker active: %s", instance.Status.Worker.CircuitBreakerReason)
	}
	return "All dependency circuit breakers are closed"
}

// SetupWithManager sets up the controller with the Manager.
func (r *CircuitBreakerController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseInstance{}).
		Named("circuitbreaker").
		Complete(r)
}
