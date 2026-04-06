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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
	"github.com/PalenaAI/langfuse-operator/internal/langfuse"
	"github.com/PalenaAI/langfuse-operator/internal/resources"
)

const (
	phasePending   = "Pending"
	phaseRunning   = "Running"
	phaseMigrating = "Migrating"
	phaseDegraded  = "Degraded"
	phaseError     = "Error"
)

// LangfuseInstanceReconciler reconciles a LangfuseInstance object
type LangfuseInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

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
		instance.Status.Phase = phasePending
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

	// 3a. Reconcile managed ClickHouse
	if instance.Spec.ClickHouse != nil && instance.Spec.ClickHouse.Managed != nil {
		if err := r.reconcileClickHouse(ctx, instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling clickhouse: %w", err)
		}
		log.Info("reconciled managed clickhouse")
	}

	// 3b. Reconcile managed Redis
	if instance.Spec.Redis != nil && instance.Spec.Redis.Managed != nil {
		if err := r.reconcileRedis(ctx, instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling redis: %w", err)
		}
		log.Info("reconciled managed redis")
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

	// 7. Reconcile networking (NetworkPolicies, Ingress, Route, HTTPRoute)
	if err := r.reconcileNetworking(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// 8. Reconcile platform resources (HPA, PDB, ServiceMonitor)
	if err := r.reconcilePlatform(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// 14. Update status
	if err := r.updateStatus(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *LangfuseInstanceReconciler) reconcileNetworking(ctx context.Context, instance *v1alpha1.LangfuseInstance) error {
	log := logf.FromContext(ctx)

	if err := r.reconcileNetworkPolicies(ctx, instance); err != nil {
		return fmt.Errorf("reconciling network policies: %w", err)
	}

	if instance.Spec.Ingress != nil && instance.Spec.Ingress.Enabled {
		ingress := resources.BuildIngress(instance)
		if err := r.reconcileIngress(ctx, instance, ingress); err != nil {
			return fmt.Errorf("reconciling ingress: %w", err)
		}
		log.Info("reconciled ingress", "name", ingress.Name)
	}

	if instance.Spec.Route != nil && instance.Spec.Route.Enabled {
		route := resources.BuildRoute(instance)
		if err := r.reconcileUnstructured(ctx, instance, route); err != nil {
			return fmt.Errorf("reconciling route: %w", err)
		}
		log.Info("reconciled openshift route", "name", route.GetName())
	}

	if instance.Spec.GatewayAPI != nil && instance.Spec.GatewayAPI.Enabled {
		httpRoute := resources.BuildHTTPRoute(instance)
		if err := r.reconcileUnstructured(ctx, instance, httpRoute); err != nil {
			return fmt.Errorf("reconciling httproute: %w", err)
		}
		log.Info("reconciled httproute", "name", httpRoute.GetName())
	}

	return nil
}

func (r *LangfuseInstanceReconciler) reconcilePlatform(ctx context.Context, instance *v1alpha1.LangfuseInstance) error {
	log := logf.FromContext(ctx)

	if err := r.reconcileHPAs(ctx, instance); err != nil {
		return fmt.Errorf("reconciling HPAs: %w", err)
	}

	if err := r.reconcilePDBs(ctx, instance); err != nil {
		return fmt.Errorf("reconciling PDBs: %w", err)
	}

	if instance.Spec.Observability != nil && instance.Spec.Observability.ServiceMonitor != nil &&
		instance.Spec.Observability.ServiceMonitor.Enabled {
		sm := resources.BuildServiceMonitor(instance)
		if err := r.reconcileUnstructured(ctx, instance, sm); err != nil {
			return fmt.Errorf("reconciling servicemonitor: %w", err)
		}
		log.Info("reconciled servicemonitor", "name", sm.GetName())
	}

	return nil
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
		instance.Status.Phase = phaseRunning
		instance.Status.Ready = true
	} else {
		instance.Status.Phase = phasePending
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
	case phaseRunning:
		return "AllComponentsReady"
	case phasePending:
		return "ComponentsStarting"
	case phaseMigrating:
		return "MigrationInProgress"
	case phaseDegraded:
		return "ComponentDegraded"
	case phaseError:
		return "ReconcileError"
	default:
		return "Unknown"
	}
}

func (r *LangfuseInstanceReconciler) reconcileNetworkPolicies(ctx context.Context, instance *v1alpha1.LangfuseInstance) error {
	log := logf.FromContext(ctx)

	// Check if NetworkPolicy is disabled
	if instance.Spec.Security != nil &&
		instance.Spec.Security.NetworkPolicy != nil &&
		instance.Spec.Security.NetworkPolicy.Enabled != nil &&
		!*instance.Spec.Security.NetworkPolicy.Enabled {
		// TODO: delete existing NetworkPolicies if they were previously created
		return nil
	}

	webNetpol := resources.BuildWebNetworkPolicy(instance)
	if err := r.reconcileNetworkPolicy(ctx, instance, webNetpol); err != nil {
		return fmt.Errorf("web network policy: %w", err)
	}
	log.Info("reconciled web network policy", "name", webNetpol.Name)

	workerNetpol := resources.BuildWorkerNetworkPolicy(instance)
	if err := r.reconcileNetworkPolicy(ctx, instance, workerNetpol); err != nil {
		return fmt.Errorf("worker network policy: %w", err)
	}
	log.Info("reconciled worker network policy", "name", workerNetpol.Name)

	return nil
}

func (r *LangfuseInstanceReconciler) reconcileIngress(ctx context.Context, instance *v1alpha1.LangfuseInstance, desired *networkingv1.Ingress) error {
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	existing := &networkingv1.Ingress{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("getting ingress: %w", err)
	}

	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) ||
		!equality.Semantic.DeepEqual(existing.Annotations, desired.Annotations) {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations
		return r.Update(ctx, existing)
	}

	return nil
}

func (r *LangfuseInstanceReconciler) reconcileUnstructured(ctx context.Context, instance *v1alpha1.LangfuseInstance, desired *unstructured.Unstructured) error {
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(desired.GroupVersionKind())
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("getting %s: %w", desired.GetKind(), err)
	}

	// Update spec if changed
	desiredSpec := desired.Object["spec"]
	existingSpec := existing.Object["spec"]
	if !equality.Semantic.DeepEqual(existingSpec, desiredSpec) {
		existing.Object["spec"] = desiredSpec
		existing.SetLabels(desired.GetLabels())
		existing.SetAnnotations(desired.GetAnnotations())
		return r.Update(ctx, existing)
	}

	return nil
}

func (r *LangfuseInstanceReconciler) reconcileNetworkPolicy(ctx context.Context, instance *v1alpha1.LangfuseInstance, desired *networkingv1.NetworkPolicy) error {
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	existing := &networkingv1.NetworkPolicy{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("getting network policy: %w", err)
	}

	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		return r.Update(ctx, existing)
	}

	return nil
}

func (r *LangfuseInstanceReconciler) reconcileClickHouse(ctx context.Context, instance *v1alpha1.LangfuseInstance) error {
	// ConfigMap
	cm := resources.BuildClickHouseConfigMap(instance)
	if err := r.reconcileConfigMap(ctx, instance, cm); err != nil {
		return fmt.Errorf("clickhouse configmap: %w", err)
	}
	// Service
	svc := resources.BuildClickHouseService(instance)
	if err := r.reconcileService(ctx, instance, svc); err != nil {
		return fmt.Errorf("clickhouse service: %w", err)
	}
	// StatefulSet
	sts := resources.BuildClickHouseStatefulSet(instance)
	if err := r.reconcileStatefulSet(ctx, instance, sts); err != nil {
		return fmt.Errorf("clickhouse statefulset: %w", err)
	}
	return nil
}

func (r *LangfuseInstanceReconciler) reconcileRedis(ctx context.Context, instance *v1alpha1.LangfuseInstance) error {
	// Service
	svc := resources.BuildRedisService(instance)
	if err := r.reconcileService(ctx, instance, svc); err != nil {
		return fmt.Errorf("redis service: %w", err)
	}
	// StatefulSet
	sts := resources.BuildRedisStatefulSet(instance)
	if err := r.reconcileStatefulSet(ctx, instance, sts); err != nil {
		return fmt.Errorf("redis statefulset: %w", err)
	}
	return nil
}

func (r *LangfuseInstanceReconciler) reconcileStatefulSet(ctx context.Context, instance *v1alpha1.LangfuseInstance, desired *appsv1.StatefulSet) error {
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	existing := &appsv1.StatefulSet{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("getting statefulset: %w", err)
	}

	if !equality.Semantic.DeepEqual(existing.Spec.Template, desired.Spec.Template) ||
		!equality.Semantic.DeepEqual(existing.Spec.Replicas, desired.Spec.Replicas) {
		existing.Spec.Template = desired.Spec.Template
		existing.Spec.Replicas = desired.Spec.Replicas
		existing.Labels = desired.Labels
		return r.Update(ctx, existing)
	}

	return nil
}

func (r *LangfuseInstanceReconciler) reconcileConfigMap(ctx context.Context, instance *v1alpha1.LangfuseInstance, desired *corev1.ConfigMap) error {
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("getting configmap: %w", err)
	}

	if !equality.Semantic.DeepEqual(existing.Data, desired.Data) {
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		return r.Update(ctx, existing)
	}

	return nil
}

func (r *LangfuseInstanceReconciler) reconcileHPAs(ctx context.Context, instance *v1alpha1.LangfuseInstance) error {
	// Web HPA
	if instance.Spec.Web.Autoscaling != nil && instance.Spec.Web.Autoscaling.Enabled {
		hpa := resources.BuildWebHPA(instance)
		if err := r.reconcileHPA(ctx, instance, hpa); err != nil {
			return fmt.Errorf("web HPA: %w", err)
		}
	}
	// Worker HPA
	if instance.Spec.Worker.Autoscaling != nil && instance.Spec.Worker.Autoscaling.Enabled {
		hpa := resources.BuildWorkerHPA(instance)
		if err := r.reconcileHPA(ctx, instance, hpa); err != nil {
			return fmt.Errorf("worker HPA: %w", err)
		}
	}
	return nil
}

func (r *LangfuseInstanceReconciler) reconcileHPA(ctx context.Context, instance *v1alpha1.LangfuseInstance, desired *autoscalingv2.HorizontalPodAutoscaler) error {
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	existing := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("getting HPA: %w", err)
	}

	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		return r.Update(ctx, existing)
	}

	return nil
}

func (r *LangfuseInstanceReconciler) reconcilePDBs(ctx context.Context, instance *v1alpha1.LangfuseInstance) error {
	// Web PDB
	if instance.Spec.Web.PodDisruptionBudget != nil && instance.Spec.Web.PodDisruptionBudget.Enabled {
		pdb := resources.BuildWebPDB(instance)
		if err := r.reconcilePDB(ctx, instance, pdb); err != nil {
			return fmt.Errorf("web PDB: %w", err)
		}
	}
	// Worker PDB
	if instance.Spec.Worker.PodDisruptionBudget != nil && instance.Spec.Worker.PodDisruptionBudget.Enabled {
		pdb := resources.BuildWorkerPDB(instance)
		if err := r.reconcilePDB(ctx, instance, pdb); err != nil {
			return fmt.Errorf("worker PDB: %w", err)
		}
	}
	return nil
}

func (r *LangfuseInstanceReconciler) reconcilePDB(ctx context.Context, instance *v1alpha1.LangfuseInstance, desired *policyv1.PodDisruptionBudget) error {
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	existing := &policyv1.PodDisruptionBudget{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("getting PDB: %w", err)
	}

	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		return r.Update(ctx, existing)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LangfuseInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseInstance{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Named("langfuseinstance")

	// Watch OpenShift Routes if the API is available
	routeGVK := schema.GroupVersionKind{Group: "route.openshift.io", Version: "v1", Kind: "Route"}
	if _, err := mgr.GetRESTMapper().RESTMapping(routeGVK.GroupKind(), routeGVK.Version); err == nil {
		route := &unstructured.Unstructured{}
		route.SetGroupVersionKind(routeGVK)
		builder = builder.Owns(route)
	}

	// Watch Gateway API HTTPRoutes if the API is available
	httpRouteGVK := schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "HTTPRoute"}
	if _, err := mgr.GetRESTMapper().RESTMapping(httpRouteGVK.GroupKind(), httpRouteGVK.Version); err == nil {
		httpRoute := &unstructured.Unstructured{}
		httpRoute.SetGroupVersionKind(httpRouteGVK)
		builder = builder.Owns(httpRoute)
	}

	return builder.Complete(r)
}
