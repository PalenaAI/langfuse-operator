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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/bitkaio/langfuse-operator/api/v1alpha1"
	"github.com/bitkaio/langfuse-operator/internal/langfuse"
	"github.com/bitkaio/langfuse-operator/internal/resources"
)

// LangfuseInstanceReconciler reconciles a LangfuseInstance object
type LangfuseInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=langfuse.bitkaio.com,resources=langfuseinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=langfuse.bitkaio.com,resources=langfuseinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=langfuse.bitkaio.com,resources=langfuseinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *LangfuseInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the LangfuseInstance CR
	instance := &v1alpha1.LangfuseInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching LangfuseInstance: %w", err)
	}

	// 2. Set initial phase
	if instance.Status.Phase == "" {
		instance.Status.Phase = "Pending"
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting initial phase: %w", err)
		}
	}

	// 3. Build env var config
	config, err := langfuse.BuildConfig(instance)
	if err != nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ConfigError",
			Message:            err.Error(),
			ObservedGeneration: instance.Generation,
		})
		if statusErr := r.Status().Update(ctx, instance); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, fmt.Errorf("building config: %w", err)
	}

	// 4. Reconcile Web Deployment
	webDeploy := resources.BuildWebDeployment(instance, config)
	if err := r.reconcileDeployment(ctx, instance, webDeploy); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling web deployment: %w", err)
	}
	log.Info("reconciled web deployment", "name", webDeploy.Name)

	// 5. Reconcile Web Service
	webSvc := resources.BuildWebService(instance)
	if err := r.reconcileService(ctx, instance, webSvc); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling web service: %w", err)
	}
	log.Info("reconciled web service", "name", webSvc.Name)

	// 6. Reconcile Worker Deployment
	workerDeploy := resources.BuildWorkerDeployment(instance, config)
	if err := r.reconcileDeployment(ctx, instance, workerDeploy); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling worker deployment: %w", err)
	}
	log.Info("reconciled worker deployment", "name", workerDeploy.Name)

	// 7. Update status
	if err := r.updateStatus(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *LangfuseInstanceReconciler) reconcileDeployment(ctx context.Context, instance *v1alpha1.LangfuseInstance, desired *appsv1.Deployment) error {
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("getting deployment: %w", err)
	}

	// Update if spec changed
	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		return r.Update(ctx, existing)
	}

	return nil
}

func (r *LangfuseInstanceReconciler) reconcileService(ctx context.Context, instance *v1alpha1.LangfuseInstance, desired *corev1.Service) error {
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("getting service: %w", err)
	}

	// Update if spec changed (preserve ClusterIP)
	if !equality.Semantic.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) ||
		!equality.Semantic.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) {
		existing.Spec.Ports = desired.Spec.Ports
		existing.Spec.Selector = desired.Spec.Selector
		existing.Labels = desired.Labels
		return r.Update(ctx, existing)
	}

	return nil
}

func (r *LangfuseInstanceReconciler) updateStatus(ctx context.Context, instance *v1alpha1.LangfuseInstance) error {
	// Fetch current deployment states
	webDeploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: resources.WebName(instance), Namespace: instance.Namespace}, webDeploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting web deployment status: %w", err)
		}
	} else {
		instance.Status.Web = &v1alpha1.ComponentStatus{
			Replicas:      webDeploy.Status.Replicas,
			ReadyReplicas: webDeploy.Status.ReadyReplicas,
			Endpoint:      fmt.Sprintf("http://%s.%s.svc:3000", resources.WebServiceName(instance), instance.Namespace),
		}
	}

	workerDeploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: resources.WorkerName(instance), Namespace: instance.Namespace}, workerDeploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting worker deployment status: %w", err)
		}
	} else {
		instance.Status.Worker = &v1alpha1.WorkerComponentStatus{
			ComponentStatus: v1alpha1.ComponentStatus{
				Replicas:      workerDeploy.Status.Replicas,
				ReadyReplicas: workerDeploy.Status.ReadyReplicas,
			},
		}
	}

	// Determine readiness — in Phase 1 just check if deployments exist
	webReady := instance.Status.Web != nil && instance.Status.Web.ReadyReplicas > 0
	workerReady := instance.Status.Worker != nil && instance.Status.Worker.ReadyReplicas > 0

	if webReady && workerReady {
		instance.Status.Phase = "Running"
		instance.Status.Ready = true
	} else {
		instance.Status.Phase = "Pending"
		instance.Status.Ready = false
	}

	instance.Status.Version = instance.Spec.Image.Tag
	instance.Status.PublicUrl = instance.Spec.Auth.NextAuthUrl

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             boolToConditionStatus(instance.Status.Ready),
		Reason:             phaseToReason(instance.Status.Phase),
		Message:            fmt.Sprintf("Phase: %s", instance.Status.Phase),
		ObservedGeneration: instance.Generation,
	})

	return r.Status().Update(ctx, instance)
}

func boolToConditionStatus(b bool) metav1.ConditionStatus {
	if b {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func phaseToReason(phase string) string {
	switch phase {
	case "Running":
		return "AllComponentsReady"
	case "Pending":
		return "ComponentsStarting"
	case "Migrating":
		return "MigrationInProgress"
	case "Degraded":
		return "ComponentDegraded"
	case "Error":
		return "ReconcileError"
	default:
		return "Unknown"
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *LangfuseInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseInstance{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("langfuseinstance").
		Complete(r)
}
