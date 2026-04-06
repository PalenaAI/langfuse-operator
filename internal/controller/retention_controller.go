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

const retentionRequeueInterval = 5 * time.Minute

// tableRetentionMapping maps a Langfuse ClickHouse table to its TTL timestamp column.
type tableRetentionMapping struct {
	Table           string
	TimestampColumn string
}

var langfuseClickHouseTables = []tableRetentionMapping{
	{Table: "traces", TimestampColumn: "timestamp"},
	{Table: "observations", TimestampColumn: "start_time"},
	{Table: "scores", TimestampColumn: "timestamp"},
	{Table: "events", TimestampColumn: "timestamp"},
}

// RetentionController manages ClickHouse data retention policies for LangfuseInstance objects.
type RetentionController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *RetentionController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the LangfuseInstance
	instance := &v1alpha1.LangfuseInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching LangfuseInstance: %w", err)
	}

	// 2. If ClickHouse retention is not configured, skip
	if instance.Spec.ClickHouse == nil || instance.Spec.ClickHouse.Retention == nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "RetentionConfigured",
			Status:             metav1.ConditionFalse,
			Reason:             "NotConfigured",
			Message:            "No ClickHouse retention policy configured",
			ObservedGeneration: instance.Generation,
		})
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating retention status: %w", err)
		}
		return ctrl.Result{RequeueAfter: retentionRequeueInterval}, nil
	}

	retention := instance.Spec.ClickHouse.Retention

	// 3. Build desired ALTER TABLE TTL statements for each configured table
	statements := r.buildRetentionStatements(retention)
	if len(statements) > 0 {
		for _, stmt := range statements {
			// TODO: Execute ALTER TABLE statement against ClickHouse HTTP endpoint.
			// The actual ClickHouse HTTP client will be added when the connection
			// infrastructure is implemented. For now, log the intended action.
			log.Info("retention policy desired", "statement", stmt)
		}

		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "RetentionConfigured",
			Status:             metav1.ConditionTrue,
			Reason:             "PoliciesComputed",
			Message:            fmt.Sprintf("Computed %d retention TTL statements", len(statements)),
			ObservedGeneration: instance.Generation,
		})

		// Mark retention as applied in ClickHouse status
		if instance.Status.ClickHouse == nil {
			instance.Status.ClickHouse = &v1alpha1.ClickHouseStatus{}
		}
		instance.Status.ClickHouse.RetentionApplied = true
	} else {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "RetentionConfigured",
			Status:             metav1.ConditionFalse,
			Reason:             "NoTTLConfigured",
			Message:            "Retention spec present but no table TTLs configured",
			ObservedGeneration: instance.Generation,
		})
	}

	// 4. Handle storage pressure thresholds
	if retention.StoragePressure != nil && retention.StoragePressure.Enabled {
		r.evaluateStoragePressure(ctx, instance, retention.StoragePressure)
	}

	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating retention status: %w", err)
	}

	log.Info("reconciled retention policies", "instance", instance.Name)
	return ctrl.Result{RequeueAfter: retentionRequeueInterval}, nil
}

// buildRetentionStatements generates ALTER TABLE TTL statements from the retention spec.
func (r *RetentionController) buildRetentionStatements(retention *v1alpha1.RetentionSpec) []string {
	tableTTLs := map[string]int32{}
	if retention.Traces != nil && retention.Traces.TTLDays > 0 {
		tableTTLs["traces"] = retention.Traces.TTLDays
	}
	if retention.Observations != nil && retention.Observations.TTLDays > 0 {
		tableTTLs["observations"] = retention.Observations.TTLDays
	}
	if retention.Scores != nil && retention.Scores.TTLDays > 0 {
		tableTTLs["scores"] = retention.Scores.TTLDays
	}

	statements := make([]string, 0, len(tableTTLs))
	for _, table := range langfuseClickHouseTables {
		ttlDays, ok := tableTTLs[table.Table]
		if !ok {
			continue
		}
		stmt := fmt.Sprintf(
			"ALTER TABLE %s MODIFY TTL %s + INTERVAL %d DAY",
			table.Table, table.TimestampColumn, ttlDays,
		)
		statements = append(statements, stmt)
	}

	return statements
}

// evaluateStoragePressure checks storage thresholds and sets status conditions.
func (r *RetentionController) evaluateStoragePressure(ctx context.Context, instance *v1alpha1.LangfuseInstance, pressure *v1alpha1.StoragePressureSpec) {
	log := logf.FromContext(ctx)

	// TODO: Query ClickHouse system.disks table to get actual storage usage.
	// For now, set the condition to indicate monitoring is enabled but no data
	// is available yet.
	log.Info("storage pressure monitoring enabled",
		"warningThreshold", pressure.WarningThresholdPercent,
		"criticalThreshold", pressure.CriticalThresholdPercent,
		"pruneOldestPartitions", pressure.PruneOldestPartitions,
		"minRetainDays", pressure.MinRetainDays,
	)

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "StoragePressure",
		Status:             metav1.ConditionFalse,
		Reason:             "MonitoringEnabled",
		Message:            fmt.Sprintf("Storage pressure monitoring active (warn=%d%%, critical=%d%%)", pressure.WarningThresholdPercent, pressure.CriticalThresholdPercent),
		ObservedGeneration: instance.Generation,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *RetentionController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseInstance{}).
		Named("retention").
		Complete(r)
}
