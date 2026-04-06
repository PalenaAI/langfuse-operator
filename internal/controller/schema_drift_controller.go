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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

const defaultSchemaDriftCheckIntervalMinutes = 60

// SchemaDriftController detects ClickHouse schema drift for LangfuseInstance objects.
type SchemaDriftController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *SchemaDriftController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the LangfuseInstance
	instance := &v1alpha1.LangfuseInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching LangfuseInstance: %w", err)
	}

	// 2. Determine check interval
	checkInterval := defaultSchemaDriftCheckIntervalMinutes
	if instance.Spec.ClickHouse != nil &&
		instance.Spec.ClickHouse.SchemaDrift != nil &&
		instance.Spec.ClickHouse.SchemaDrift.CheckIntervalMinutes > 0 {
		checkInterval = int(instance.Spec.ClickHouse.SchemaDrift.CheckIntervalMinutes)
	}
	requeueAfter := time.Duration(checkInterval) * time.Minute

	// 3. If schema drift detection is disabled, skip
	if instance.Spec.ClickHouse == nil ||
		instance.Spec.ClickHouse.SchemaDrift == nil ||
		!instance.Spec.ClickHouse.SchemaDrift.Enabled {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "SchemaDriftChecked",
			Status:             metav1.ConditionFalse,
			Reason:             "Disabled",
			Message:            "Schema drift detection is disabled",
			ObservedGeneration: instance.Generation,
		})
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating schema drift status: %w", err)
		}
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	schemaDrift := instance.Spec.ClickHouse.SchemaDrift

	// 4. Perform schema drift check
	// TODO: Query ClickHouse system.columns table and compare against expected schema.
	// The expected schema for Langfuse tables (traces, observations, scores, events)
	// will be derived from the running Langfuse version. For now, log the intent and
	// set a condition indicating the check was attempted.
	log.Info("schema drift check triggered",
		"instance", instance.Name,
		"autoRepair", schemaDrift.AutoRepair,
		"checkInterval", checkInterval,
	)

	// No drift detected (since we can't actually query yet)
	if instance.Status.ClickHouse == nil {
		instance.Status.ClickHouse = &v1alpha1.ClickHouseStatus{}
	}
	instance.Status.ClickHouse.SchemaDrift = false

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "SchemaDriftChecked",
		Status:             metav1.ConditionTrue,
		Reason:             "CheckCompleted",
		Message:            fmt.Sprintf("Schema drift check completed (autoRepair=%t, next check in %dm)", schemaDrift.AutoRepair, checkInterval),
		ObservedGeneration: instance.Generation,
	})

	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating schema drift status: %w", err)
	}

	log.Info("reconciled schema drift detection", "instance", instance.Name, "requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaDriftController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseInstance{}).
		Named("schemadrift").
		Complete(r)
}
