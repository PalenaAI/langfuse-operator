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
	"strconv"
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

	if !instance.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
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

	// 3. Build and apply the ALTER TABLE TTL statements for each configured table
	statements := r.buildRetentionStatements(retention)
	if len(statements) > 0 {
		if instance.Status.ClickHouse == nil {
			instance.Status.ClickHouse = &v1alpha1.ClickHouseStatus{}
		}

		applied, err := r.applyRetention(ctx, instance, statements)
		// RetentionApplied reflects what ClickHouse actually accepted. It was
		// previously set to true purely from computing the statements, so the CR
		// claimed retention was active while data was retained forever.
		instance.Status.ClickHouse.RetentionApplied = err == nil

		switch {
		case err != nil:
			log.Error(err, "failed to apply retention policies", "applied", applied, "total", len(statements))
			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:   "RetentionConfigured",
				Status: metav1.ConditionFalse,
				Reason: "ApplyFailed",
				Message: fmt.Sprintf("Applied %d/%d retention TTL statements: %v",
					applied, len(statements), err),
				ObservedGeneration: instance.Generation,
			})
		default:
			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:               "RetentionConfigured",
				Status:             metav1.ConditionTrue,
				Reason:             "PoliciesApplied",
				Message:            fmt.Sprintf("Applied %d retention TTL statements", applied),
				ObservedGeneration: instance.Generation,
			})
		}
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

// applyRetention executes the TTL statements against ClickHouse, returning how
// many succeeded. It stops at the first failure so a broken statement doesn't
// mask itself behind later successes.
func (r *RetentionController) applyRetention(ctx context.Context, instance *v1alpha1.LangfuseInstance, statements []string) (int, error) {
	log := logf.FromContext(ctx)

	ch, err := newClickHouseClient(ctx, r.Client, instance)
	if err != nil {
		return 0, err
	}

	applied := 0
	for _, stmt := range statements {
		if _, err := ch.exec(ctx, stmt); err != nil {
			return applied, fmt.Errorf("executing %q: %w", stmt, err)
		}
		log.V(1).Info("applied retention policy", "statement", stmt)
		applied++
	}
	return applied, nil
}

// evaluateStoragePressure queries ClickHouse disk usage, records it on the
// status, and raises a condition when the configured thresholds are crossed.
func (r *RetentionController) evaluateStoragePressure(ctx context.Context, instance *v1alpha1.LangfuseInstance, pressure *v1alpha1.StoragePressureSpec) {
	log := logf.FromContext(ctx)

	used, total, err := r.queryDiskUsage(ctx, instance)
	if err != nil {
		log.Error(err, "failed to query ClickHouse storage usage")
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "StoragePressure",
			Status:             metav1.ConditionUnknown,
			Reason:             "QueryFailed",
			Message:            fmt.Sprintf("Cannot read ClickHouse disk usage: %v", err),
			ObservedGeneration: instance.Generation,
		})
		return
	}

	if instance.Status.ClickHouse == nil {
		instance.Status.ClickHouse = &v1alpha1.ClickHouseStatus{}
	}
	instance.Status.ClickHouse.StorageUsed = humanizeBytes(used)
	instance.Status.ClickHouse.StorageTotal = humanizeBytes(total)

	if total == 0 {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "StoragePressure",
			Status:             metav1.ConditionUnknown,
			Reason:             "NoCapacityReported",
			Message:            "ClickHouse reported zero total disk capacity",
			ObservedGeneration: instance.Generation,
		})
		return
	}

	usedPercent := int32(used * 100 / total)
	warn, critical := pressure.WarningThresholdPercent, pressure.CriticalThresholdPercent
	summary := fmt.Sprintf("ClickHouse storage %d%% used (%s of %s; warn=%d%%, critical=%d%%)",
		usedPercent, humanizeBytes(used), humanizeBytes(total), warn, critical)

	switch {
	case critical > 0 && usedPercent >= critical:
		// Pruning is intentionally not automated here: dropping partitions is
		// irreversible data loss, so the operator reports and lets a human act.
		// spec.storagePressure.pruneOldestPartitions remains unimplemented.
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "StoragePressure",
			Status:             metav1.ConditionTrue,
			Reason:             "CriticalThresholdExceeded",
			Message:            summary,
			ObservedGeneration: instance.Generation,
		})
	case warn > 0 && usedPercent >= warn:
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "StoragePressure",
			Status:             metav1.ConditionTrue,
			Reason:             "WarningThresholdExceeded",
			Message:            summary,
			ObservedGeneration: instance.Generation,
		})
	default:
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "StoragePressure",
			Status:             metav1.ConditionFalse,
			Reason:             "WithinThresholds",
			Message:            summary,
			ObservedGeneration: instance.Generation,
		})
	}
}

// queryDiskUsage returns used and total bytes across ClickHouse's disks.
func (r *RetentionController) queryDiskUsage(ctx context.Context, instance *v1alpha1.LangfuseInstance) (uint64, uint64, error) {
	ch, err := newClickHouseClient(ctx, r.Client, instance)
	if err != nil {
		return 0, 0, err
	}

	rows, err := ch.queryRows(ctx,
		"SELECT sum(total_space - free_space), sum(total_space) FROM system.disks")
	if err != nil {
		return 0, 0, err
	}
	if len(rows) == 0 {
		return 0, 0, fmt.Errorf("system.disks returned no rows")
	}

	fields := strings.Split(rows[0], "\t")
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected system.disks response %q", rows[0])
	}
	used, err := strconv.ParseUint(strings.TrimSpace(fields[0]), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing used bytes: %w", err)
	}
	total, err := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing total bytes: %w", err)
	}
	return used, total, nil
}

// humanizeBytes renders a byte count using binary units, matching how
// Kubernetes resource quantities read (e.g. "12.3Gi").
func humanizeBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ci", float64(b)/float64(div), "KMGTP"[exp])
}

// SetupWithManager sets up the controller with the Manager.
func (r *RetentionController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseInstance{}).
		Named("retention").
		Complete(r)
}
