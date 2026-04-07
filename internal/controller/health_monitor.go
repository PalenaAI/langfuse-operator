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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
	"github.com/PalenaAI/langfuse-operator/internal/resources"
)

const (
	healthCheckInterval = 30 * time.Second

	// Condition types for health monitoring.
	conditionDatabaseReady    = "DatabaseReady"
	conditionClickHouseReady  = "ClickHouseReady"
	conditionRedisReady       = "RedisReady"
	conditionBlobStorageReady = "BlobStorageReady"
	conditionWebReady         = "WebReady"
	conditionWorkerReady      = "WorkerReady"
)

// HealthMonitorReconciler periodically checks the health of all components
// in a LangfuseInstance and updates status conditions accordingly.
type HealthMonitorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *HealthMonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the LangfuseInstance.
	instance := &v1alpha1.LangfuseInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching LangfuseInstance: %w", err)
	}

	// 2. Skip health checks if instance is not in Running or Degraded phase.
	if instance.Status.Phase != phaseRunning && instance.Status.Phase != phaseDegraded {
		log.V(1).Info("skipping health check, instance not in Running or Degraded phase",
			"phase", instance.Status.Phase)
		return ctrl.Result{RequeueAfter: healthCheckInterval}, nil
	}

	log.V(1).Info("running health checks", "instance", instance.Name)

	// 3. Check PostgreSQL connectivity.
	dbCondition := r.checkDatabase(ctx, instance)
	r.applyCondition(instance, dbCondition)

	// 4. Check ClickHouse connectivity.
	chCondition := r.checkClickHouse(ctx, instance)
	r.applyCondition(instance, chCondition)

	// 5. Check Redis connectivity.
	redisCondition := r.checkRedis(ctx, instance)
	r.applyCondition(instance, redisCondition)

	// 6. Check blob storage.
	blobCondition := r.checkBlobStorage(ctx, instance)
	r.applyCondition(instance, blobCondition)

	// 7. Check Web deployment health.
	webCondition := r.checkWebDeployment(ctx, instance)
	r.applyCondition(instance, webCondition)

	// 8. Check Worker deployment health.
	workerCondition := r.checkWorkerDeployment(ctx, instance)
	r.applyCondition(instance, workerCondition)

	// 9. Determine overall health.
	r.determineOverallHealth(instance, []metav1.Condition{
		dbCondition,
		chCondition,
		redisCondition,
		blobCondition,
		webCondition,
		workerCondition,
	})

	// 10. Update status.
	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating health status: %w", err)
	}

	return ctrl.Result{RequeueAfter: healthCheckInterval}, nil
}

// checkDatabase evaluates PostgreSQL health from the existing status.
func (r *HealthMonitorReconciler) checkDatabase(ctx context.Context, instance *v1alpha1.LangfuseInstance) metav1.Condition {
	log := logf.FromContext(ctx)

	if instance.Status.Database != nil && instance.Status.Database.Connected {
		return metav1.Condition{
			Type:               conditionDatabaseReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Connected",
			Message:            "PostgreSQL database is connected",
			ObservedGeneration: instance.Generation,
		}
	}

	// TODO: implement real TCP/HTTP probe to PostgreSQL
	log.V(1).Info("database health check: status not set or not connected, would probe PostgreSQL endpoint")

	return metav1.Condition{
		Type:               conditionDatabaseReady,
		Status:             metav1.ConditionFalse,
		Reason:             "NotConnected",
		Message:            "PostgreSQL database status is not available or not connected",
		ObservedGeneration: instance.Generation,
	}
}

// checkClickHouse evaluates ClickHouse health from the existing status.
func (r *HealthMonitorReconciler) checkClickHouse(ctx context.Context, instance *v1alpha1.LangfuseInstance) metav1.Condition {
	log := logf.FromContext(ctx)

	if instance.Status.ClickHouse != nil && instance.Status.ClickHouse.Connected {
		return metav1.Condition{
			Type:               conditionClickHouseReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Connected",
			Message:            "ClickHouse is connected",
			ObservedGeneration: instance.Generation,
		}
	}

	log.V(1).Info("clickhouse health check: status not set or not connected, would probe ClickHouse endpoint")

	return metav1.Condition{
		Type:               conditionClickHouseReady,
		Status:             metav1.ConditionFalse,
		Reason:             "NotConnected",
		Message:            "ClickHouse status is not available or not connected",
		ObservedGeneration: instance.Generation,
	}
}

// checkRedis evaluates Redis health from the existing status.
func (r *HealthMonitorReconciler) checkRedis(ctx context.Context, instance *v1alpha1.LangfuseInstance) metav1.Condition {
	log := logf.FromContext(ctx)

	if instance.Status.Redis != nil && instance.Status.Redis.Connected {
		return metav1.Condition{
			Type:               conditionRedisReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Connected",
			Message:            "Redis is connected",
			ObservedGeneration: instance.Generation,
		}
	}

	log.V(1).Info("redis health check: status not set or not connected, would probe Redis endpoint")

	return metav1.Condition{
		Type:               conditionRedisReady,
		Status:             metav1.ConditionFalse,
		Reason:             "NotConnected",
		Message:            "Redis status is not available or not connected",
		ObservedGeneration: instance.Generation,
	}
}

// checkBlobStorage evaluates blob storage health from the existing status.
func (r *HealthMonitorReconciler) checkBlobStorage(ctx context.Context, instance *v1alpha1.LangfuseInstance) metav1.Condition {
	log := logf.FromContext(ctx)

	// If blob storage is not configured, treat it as healthy (it's optional).
	if instance.Spec.BlobStorage == nil {
		return metav1.Condition{
			Type:               conditionBlobStorageReady,
			Status:             metav1.ConditionTrue,
			Reason:             "NotConfigured",
			Message:            "Blob storage is not configured (using default)",
			ObservedGeneration: instance.Generation,
		}
	}

	if instance.Status.BlobStorage != nil && instance.Status.BlobStorage.Connected {
		return metav1.Condition{
			Type:               conditionBlobStorageReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Connected",
			Message:            fmt.Sprintf("Blob storage is connected (provider: %s)", instance.Status.BlobStorage.Provider),
			ObservedGeneration: instance.Generation,
		}
	}

	log.V(1).Info("blob storage health check: status not set or not connected, would probe blob storage endpoint")

	return metav1.Condition{
		Type:               conditionBlobStorageReady,
		Status:             metav1.ConditionFalse,
		Reason:             "NotConnected",
		Message:            "Blob storage status is not available or not connected",
		ObservedGeneration: instance.Generation,
	}
}

// checkWebDeployment evaluates Web component health from deployment readiness.
func (r *HealthMonitorReconciler) checkWebDeployment(ctx context.Context, instance *v1alpha1.LangfuseInstance) metav1.Condition {
	log := logf.FromContext(ctx)

	deploy := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      resources.WebName(instance),
		Namespace: instance.Namespace,
	}, deploy)

	if apierrors.IsNotFound(err) {
		log.V(1).Info("web deployment not found")
		return metav1.Condition{
			Type:               conditionWebReady,
			Status:             metav1.ConditionFalse,
			Reason:             "DeploymentNotFound",
			Message:            "Web deployment does not exist",
			ObservedGeneration: instance.Generation,
		}
	}
	if err != nil {
		log.Error(err, "failed to get web deployment")
		return metav1.Condition{
			Type:               conditionWebReady,
			Status:             metav1.ConditionFalse,
			Reason:             "FetchError",
			Message:            fmt.Sprintf("Failed to check web deployment: %v", err),
			ObservedGeneration: instance.Generation,
		}
	}

	if deploy.Status.ReadyReplicas > 0 && deploy.Status.ReadyReplicas >= deploy.Status.Replicas {
		return metav1.Condition{
			Type:               conditionWebReady,
			Status:             metav1.ConditionTrue,
			Reason:             "DeploymentReady",
			Message:            fmt.Sprintf("Web deployment has %d/%d ready replicas", deploy.Status.ReadyReplicas, deploy.Status.Replicas),
			ObservedGeneration: instance.Generation,
		}
	}

	return metav1.Condition{
		Type:               conditionWebReady,
		Status:             metav1.ConditionFalse,
		Reason:             "DeploymentNotReady",
		Message:            fmt.Sprintf("Web deployment has %d/%d ready replicas", deploy.Status.ReadyReplicas, deploy.Status.Replicas),
		ObservedGeneration: instance.Generation,
	}
}

// checkWorkerDeployment evaluates Worker component health from deployment readiness.
func (r *HealthMonitorReconciler) checkWorkerDeployment(ctx context.Context, instance *v1alpha1.LangfuseInstance) metav1.Condition {
	log := logf.FromContext(ctx)

	deploy := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      resources.WorkerName(instance),
		Namespace: instance.Namespace,
	}, deploy)

	if apierrors.IsNotFound(err) {
		log.V(1).Info("worker deployment not found")
		return metav1.Condition{
			Type:               conditionWorkerReady,
			Status:             metav1.ConditionFalse,
			Reason:             "DeploymentNotFound",
			Message:            "Worker deployment does not exist",
			ObservedGeneration: instance.Generation,
		}
	}
	if err != nil {
		log.Error(err, "failed to get worker deployment")
		return metav1.Condition{
			Type:               conditionWorkerReady,
			Status:             metav1.ConditionFalse,
			Reason:             "FetchError",
			Message:            fmt.Sprintf("Failed to check worker deployment: %v", err),
			ObservedGeneration: instance.Generation,
		}
	}

	if deploy.Status.ReadyReplicas > 0 && deploy.Status.ReadyReplicas >= deploy.Status.Replicas {
		return metav1.Condition{
			Type:               conditionWorkerReady,
			Status:             metav1.ConditionTrue,
			Reason:             "DeploymentReady",
			Message:            fmt.Sprintf("Worker deployment has %d/%d ready replicas", deploy.Status.ReadyReplicas, deploy.Status.Replicas),
			ObservedGeneration: instance.Generation,
		}
	}

	return metav1.Condition{
		Type:               conditionWorkerReady,
		Status:             metav1.ConditionFalse,
		Reason:             "DeploymentNotReady",
		Message:            fmt.Sprintf("Worker deployment has %d/%d ready replicas", deploy.Status.ReadyReplicas, deploy.Status.Replicas),
		ObservedGeneration: instance.Generation,
	}
}

// applyCondition sets the condition on the instance and emits an event if the condition changed.
func (r *HealthMonitorReconciler) applyCondition(instance *v1alpha1.LangfuseInstance, condition metav1.Condition) {
	existing := meta.FindStatusCondition(instance.Status.Conditions, condition.Type)
	if existing != nil && conditionChanged(*existing, condition) {
		eventType := "Normal"
		if condition.Status == metav1.ConditionFalse {
			eventType = "Warning"
		}
		r.Recorder.Event(instance, eventType, condition.Reason,
			fmt.Sprintf("%s: %s", condition.Type, condition.Message))
	}

	meta.SetStatusCondition(&instance.Status.Conditions, condition)
}

// determineOverallHealth sets the instance phase based on component conditions.
// Critical components are Database, ClickHouse, Redis, Web, and Worker.
// BlobStorage is only critical if explicitly configured.
func (r *HealthMonitorReconciler) determineOverallHealth(instance *v1alpha1.LangfuseInstance, conditions []metav1.Condition) {
	allReady := true
	for _, c := range conditions {
		if c.Status != metav1.ConditionTrue {
			allReady = false
			break
		}
	}

	if allReady {
		instance.Status.Phase = phaseRunning
		instance.Status.Ready = true
	} else {
		instance.Status.Phase = phaseDegraded
		instance.Status.Ready = false
	}
}

// conditionChanged reports whether a condition has meaningfully changed
// (different status or reason).
func conditionChanged(existing, updated metav1.Condition) bool {
	return existing.Status != updated.Status || existing.Reason != updated.Reason
}

// SetupWithManager sets up the health monitor controller with the Manager.
func (r *HealthMonitorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseInstance{}).
		Named("healthmonitor").
		Complete(r)
}
