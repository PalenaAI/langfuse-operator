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
	"strings"
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

	if !instance.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
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

	// 4. Perform the schema drift check against ClickHouse.
	log.V(1).Info("running schema drift check",
		"instance", instance.Name,
		"autoRepair", schemaDrift.AutoRepair,
		"checkInterval", checkInterval,
	)

	if instance.Status.ClickHouse == nil {
		instance.Status.ClickHouse = &v1alpha1.ClickHouseStatus{}
	}

	missing, err := r.findMissingTables(ctx, instance)
	switch {
	case err != nil:
		// Unknown, not "no drift" — the check previously reported success
		// without ever querying ClickHouse.
		log.Error(err, "schema drift check failed")
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "SchemaDriftChecked",
			Status:             metav1.ConditionUnknown,
			Reason:             "CheckFailed",
			Message:            fmt.Sprintf("Cannot inspect ClickHouse schema: %v", err),
			ObservedGeneration: instance.Generation,
		})

	case len(missing) > 0:
		instance.Status.ClickHouse.SchemaDrift = true
		// autoRepair is deliberately not acted on: recreating Langfuse's tables
		// belongs to its own migrations, and a wrong DDL here would corrupt the
		// schema. Missing tables almost always mean migrations have not run.
		repairNote := ""
		if schemaDrift.AutoRepair {
			repairNote = " autoRepair cannot fix this — Langfuse owns its schema; check that migrations completed."
		}
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:   "SchemaDriftChecked",
			Status: metav1.ConditionFalse,
			Reason: "TablesMissing",
			Message: fmt.Sprintf("Expected Langfuse tables missing from ClickHouse database %q: %s.%s",
				defaultClickHouseDatabase, strings.Join(missing, ", "), repairNote),
			ObservedGeneration: instance.Generation,
		})

	default:
		instance.Status.ClickHouse.SchemaDrift = false
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:   "SchemaDriftChecked",
			Status: metav1.ConditionTrue,
			Reason: "NoDriftDetected",
			Message: fmt.Sprintf("All %d expected Langfuse tables present in database %q (next check in %dm)",
				len(expectedClickHouseTables), defaultClickHouseDatabase, checkInterval),
			ObservedGeneration: instance.Generation,
		})
	}

	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating schema drift status: %w", err)
	}

	log.Info("reconciled schema drift detection", "instance", instance.Name, "requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// expectedClickHouseTables are the core tables Langfuse's ClickHouse migrations
// create. Their absence is the drift that actually matters in practice — it
// means migrations never ran, ran against a different database, or the schema
// was partially dropped.
//
// The check is deliberately table-level, not column-level: Langfuse owns its
// schema and changes it between versions, so an operator-side column manifest
// would produce false drift on every upgrade.
var expectedClickHouseTables = []string{
	"traces",
	"observations",
	"scores",
	"schema_migrations",
}

// findMissingTables returns the expected tables absent from ClickHouse.
func (r *SchemaDriftController) findMissingTables(ctx context.Context, instance *v1alpha1.LangfuseInstance) ([]string, error) {
	ch, err := newClickHouseClient(ctx, r.Client, instance)
	if err != nil {
		return nil, err
	}

	rows, err := ch.queryRows(ctx, fmt.Sprintf(
		"SELECT name FROM system.tables WHERE database = '%s'", defaultClickHouseDatabase))
	if err != nil {
		return nil, err
	}

	present := make(map[string]bool, len(rows))
	for _, row := range rows {
		present[strings.TrimSpace(row)] = true
	}

	var missing []string
	for _, table := range expectedClickHouseTables {
		if !present[table] {
			missing = append(missing, table)
		}
	}
	return missing, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaDriftController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseInstance{}).
		Named("schemadrift").
		Complete(r)
}
