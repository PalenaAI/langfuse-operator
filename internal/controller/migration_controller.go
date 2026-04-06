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

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
	"github.com/PalenaAI/langfuse-operator/internal/langfuse"
	"github.com/PalenaAI/langfuse-operator/internal/resources"
)

// MigrationController watches LangfuseInstance for version changes and manages migration Jobs.
type MigrationController struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *MigrationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	instance := &v1alpha1.LangfuseInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip if migration is disabled
	if instance.Spec.Database != nil && instance.Spec.Database.Migration != nil &&
		instance.Spec.Database.Migration.RunOnDeploy != nil && !*instance.Spec.Database.Migration.RunOnDeploy {
		return ctrl.Result{}, nil
	}

	// Check if version changed
	desiredTag := instance.Spec.Image.Tag
	currentVersion := instance.Status.Version
	if !langfuse.VersionChanged(desiredTag, currentVersion) && currentVersion != "" {
		// Version hasn't changed, check if existing migration job needs cleanup
		return r.cleanupCompletedJobs(ctx, instance)
	}

	// Build config for migration job env vars
	config, err := langfuse.BuildConfig(instance)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building config for migration: %w", err)
	}

	// Check for existing migration job
	jobName := resources.MigrationJobName(instance)
	existingJob := &batchv1.Job{}
	err = r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: instance.Namespace}, existingJob)

	if apierrors.IsNotFound(err) {
		// Create migration job
		log.Info("creating migration job", "version", desiredTag)
		job := resources.BuildMigrationJob(instance, config)
		if err := controllerutil.SetControllerReference(instance, job, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting owner reference on migration job: %w", err)
		}

		// Set instance phase to Migrating
		instance.Status.Phase = phaseMigrating
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "MigrationsComplete",
			Status:             metav1.ConditionFalse,
			Reason:             "MigrationStarted",
			Message:            fmt.Sprintf("Running migrations for version %s", desiredTag),
			ObservedGeneration: instance.Generation,
		})
		if statusErr := r.Status().Update(ctx, instance); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}

		if err := r.Create(ctx, job); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating migration job: %w", err)
		}
		r.Recorder.Eventf(instance, "Normal", "MigrationStarted",
			"Started migration job %s for version %s", jobName, desiredTag)

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting migration job: %w", err)
	}

	// Job exists — check status
	if existingJob.Status.Succeeded > 0 {
		log.Info("migration job succeeded", "version", desiredTag)
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "MigrationsComplete",
			Status:             metav1.ConditionTrue,
			Reason:             "MigrationSucceeded",
			Message:            fmt.Sprintf("Migrations completed for version %s", desiredTag),
			ObservedGeneration: instance.Generation,
		})
		if instance.Status.Database == nil {
			instance.Status.Database = &v1alpha1.DatabaseStatus{}
		}
		instance.Status.Database.MigrationVersion = langfuse.NormalizeVersion(desiredTag)
		if statusErr := r.Status().Update(ctx, instance); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		r.Recorder.Eventf(instance, "Normal", "MigrationCompleted",
			"Migration job %s completed successfully", jobName)
		return ctrl.Result{}, nil
	}

	if existingJob.Status.Failed > 0 {
		backoffLimit := int32(3)
		if existingJob.Spec.BackoffLimit != nil {
			backoffLimit = *existingJob.Spec.BackoffLimit
		}
		if existingJob.Status.Failed >= backoffLimit {
			log.Error(nil, "migration job failed", "version", desiredTag, "failures", existingJob.Status.Failed)
			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:               "MigrationsComplete",
				Status:             metav1.ConditionFalse,
				Reason:             "MigrationFailed",
				Message:            fmt.Sprintf("Migration job failed after %d attempts", existingJob.Status.Failed),
				ObservedGeneration: instance.Generation,
			})
			instance.Status.Phase = phaseError
			if statusErr := r.Status().Update(ctx, instance); statusErr != nil {
				log.Error(statusErr, "failed to update status")
			}
			r.Recorder.Eventf(instance, "Warning", "MigrationFailed",
				"Migration job %s failed after %d attempts", jobName, existingJob.Status.Failed)
			return ctrl.Result{}, nil
		}
	}

	// Job still running — requeue
	log.Info("migration job in progress", "version", desiredTag)
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *MigrationController) cleanupCompletedJobs(ctx context.Context, instance *v1alpha1.LangfuseInstance) (ctrl.Result, error) {
	jobName := resources.MigrationJobName(instance)
	existingJob := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: instance.Namespace}, existingJob)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("checking migration job: %w", err)
	}

	// Clean up completed jobs older than TTL
	if existingJob.Status.Succeeded > 0 && existingJob.Status.CompletionTime != nil {
		if time.Since(existingJob.Status.CompletionTime.Time) > time.Hour {
			propagation := metav1.DeletePropagationBackground
			if err := r.Delete(ctx, existingJob, &client.DeleteOptions{
				PropagationPolicy: &propagation,
			}); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("cleaning up migration job: %w", err)
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseInstance{}).
		Owns(&batchv1.Job{}).
		Named("migration").
		Complete(r)
}
