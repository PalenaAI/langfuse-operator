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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
	"github.com/PalenaAI/langfuse-operator/internal/langfuse"
)

const (
	projectFinalizer    = "langfuse.palena.ai/project-cleanup"
	projectResyncPeriod = 5 * time.Minute

	// deleteOnRemoveAnnotation controls whether the Langfuse project is deleted
	// when the LangfuseProject CR is removed.
	deleteOnRemoveAnnotation = "langfuse.palena.ai/delete-on-remove"
)

// LangfuseProjectReconciler reconciles a LangfuseProject object
type LangfuseProjectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseprojects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseprojects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseprojects/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile synchronizes the desired LangfuseProject state with the Langfuse instance.
func (r *LangfuseProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the LangfuseProject CR.
	project := &v1alpha1.LangfuseProject{}
	if err := r.Get(ctx, req.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching LangfuseProject: %w", err)
	}

	// 2. Handle deletion.
	if !project.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, project)
	}

	// 3. Ensure finalizer is set.
	if !controllerutil.ContainsFinalizer(project, projectFinalizer) {
		controllerutil.AddFinalizer(project, projectFinalizer)
		if err := r.Update(ctx, project); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
	}

	// 4. Resolve the parent LangfuseInstance.
	instance, err := r.resolveInstance(ctx, project)
	if err != nil {
		meta.SetStatusCondition(&project.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "InstanceNotReady",
			Message:            err.Error(),
			ObservedGeneration: project.Generation,
		})
		project.Status.Ready = false
		if statusErr := r.Status().Update(ctx, project); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// 5. Resolve the parent LangfuseOrganization.
	orgID, err := r.resolveOrganization(ctx, project)
	if err != nil {
		meta.SetStatusCondition(&project.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "OrganizationNotReady",
			Message:            err.Error(),
			ObservedGeneration: project.Generation,
		})
		project.Status.Ready = false
		if statusErr := r.Status().Update(ctx, project); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	project.Status.OrganizationID = orgID

	// 6. Build the Langfuse API client.
	lfClient, err := buildLangfuseClient(ctx, r.Client, instance)
	if err != nil {
		meta.SetStatusCondition(&project.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ClientError",
			Message:            err.Error(),
			ObservedGeneration: project.Generation,
		})
		project.Status.Ready = false
		if statusErr := r.Status().Update(ctx, project); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// 7. Sync project.
	projectID, err := r.syncProject(ctx, lfClient, project, orgID)
	if err != nil {
		meta.SetStatusCondition(&project.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "SyncFailed",
			Message:            fmt.Sprintf("Failed to sync project: %s", err.Error()),
			ObservedGeneration: project.Generation,
		})
		project.Status.Ready = false
		if statusErr := r.Status().Update(ctx, project); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, fmt.Errorf("syncing project: %w", err)
	}
	project.Status.ProjectID = projectID

	// 8. Sync API keys.
	apiKeyStatuses, err := r.syncAPIKeys(ctx, lfClient, project, instance)
	if err != nil {
		meta.SetStatusCondition(&project.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "APIKeySyncFailed",
			Message:            fmt.Sprintf("Failed to sync API keys: %s", err.Error()),
			ObservedGeneration: project.Generation,
		})
		project.Status.Ready = false
		if statusErr := r.Status().Update(ctx, project); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, fmt.Errorf("syncing API keys: %w", err)
	}
	project.Status.APIKeys = apiKeyStatuses

	// 9. Set Ready condition.
	meta.SetStatusCondition(&project.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Synced",
		Message:            "Project is synced with Langfuse",
		ObservedGeneration: project.Generation,
	})
	project.Status.Ready = true

	if err := r.Status().Update(ctx, project); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	log.Info("reconciled project",
		"projectId", projectID,
		"organizationId", orgID,
		"apiKeys", len(apiKeyStatuses),
	)

	return ctrl.Result{RequeueAfter: projectResyncPeriod}, nil
}

// handleDeletion processes the deletion of a LangfuseProject.
func (r *LangfuseProjectReconciler) handleDeletion(ctx context.Context, project *v1alpha1.LangfuseProject) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(project, projectFinalizer) {
		return ctrl.Result{}, nil
	}

	// Attempt cleanup via Langfuse API (best effort).
	if project.Status.ProjectID != "" {
		instance, err := r.resolveInstance(ctx, project)
		if err != nil {
			log.Error(err, "could not resolve instance during deletion, skipping Langfuse cleanup")
		} else {
			lfClient, err := buildLangfuseClient(ctx, r.Client, instance)
			if err != nil {
				log.Error(err, "could not build Langfuse client during deletion, skipping Langfuse cleanup")
			} else {
				// Revoke API keys (best effort).
				r.revokeAPIKeys(ctx, lfClient, project)

				// Delete the project in Langfuse if annotated.
				if project.Annotations[deleteOnRemoveAnnotation] == "true" {
					if err := lfClient.DeleteProject(ctx, project.Status.ProjectID); err != nil {
						log.Error(err, "failed to delete project in Langfuse (best effort)", "projectId", project.Status.ProjectID)
					} else {
						log.Info("deleted project in Langfuse", "projectId", project.Status.ProjectID)
					}
				}
			}
		}
	}

	// Delete API key Secrets explicitly (owner references handle most cases,
	// but we clean up proactively for clarity).
	for _, keySpec := range project.Spec.APIKeys {
		secret := &corev1.Secret{}
		key := types.NamespacedName{Name: keySpec.SecretName, Namespace: project.Namespace}
		if err := r.Get(ctx, key, secret); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "failed to get API key secret during deletion", "secret", key)
			}
			continue
		}
		if err := r.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "failed to delete API key secret during deletion", "secret", key)
		}
	}

	// Remove finalizer.
	controllerutil.RemoveFinalizer(project, projectFinalizer)
	if err := r.Update(ctx, project); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}

	return ctrl.Result{}, nil
}

// revokeAPIKeys attempts to revoke all managed API keys via the Langfuse API.
func (r *LangfuseProjectReconciler) revokeAPIKeys(ctx context.Context, lfClient langfuse.Client, project *v1alpha1.LangfuseProject) {
	log := logf.FromContext(ctx)

	apiKeys, err := lfClient.ListAPIKeys(ctx, project.Status.ProjectID)
	if err != nil {
		log.Error(err, "failed to list API keys for revocation", "projectId", project.Status.ProjectID)
		return
	}

	// Build a set of public keys we manage by reading the corresponding Secrets.
	managedPublicKeys := make(map[string]bool)
	for _, keySpec := range project.Spec.APIKeys {
		secret := &corev1.Secret{}
		key := types.NamespacedName{Name: keySpec.SecretName, Namespace: project.Namespace}
		if err := r.Get(ctx, key, secret); err != nil {
			continue
		}
		if pk := string(secret.Data["publicKey"]); pk != "" {
			managedPublicKeys[pk] = true
		}
	}

	for _, ak := range apiKeys {
		if managedPublicKeys[ak.PublicKey] {
			if err := lfClient.DeleteAPIKey(ctx, ak.ID); err != nil {
				log.Error(err, "failed to revoke API key (best effort)", "apiKeyId", ak.ID)
			} else {
				log.Info("revoked API key", "apiKeyId", ak.ID)
			}
		}
	}
}

// resolveInstance fetches the LangfuseInstance referenced by the project and verifies it is ready.
func (r *LangfuseProjectReconciler) resolveInstance(ctx context.Context, project *v1alpha1.LangfuseProject) (*v1alpha1.LangfuseInstance, error) {
	ns := project.Spec.InstanceRef.Namespace
	if ns == "" {
		ns = project.Namespace
	}

	instance := &v1alpha1.LangfuseInstance{}
	key := types.NamespacedName{Name: project.Spec.InstanceRef.Name, Namespace: ns}
	if err := r.Get(ctx, key, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("LangfuseInstance %q not found", key)
		}
		return nil, fmt.Errorf("fetching LangfuseInstance %q: %w", key, err)
	}

	if !instance.Status.Ready {
		return nil, fmt.Errorf("LangfuseInstance %q is not ready", key)
	}

	return instance, nil
}

// resolveOrganization fetches the LangfuseOrganization and returns its Langfuse organization ID.
func (r *LangfuseProjectReconciler) resolveOrganization(ctx context.Context, project *v1alpha1.LangfuseProject) (string, error) {
	ns := project.Spec.OrganizationRef.Namespace
	if ns == "" {
		ns = project.Namespace
	}

	org := &v1alpha1.LangfuseOrganization{}
	key := types.NamespacedName{Name: project.Spec.OrganizationRef.Name, Namespace: ns}
	if err := r.Get(ctx, key, org); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("LangfuseOrganization %q not found", key)
		}
		return "", fmt.Errorf("fetching LangfuseOrganization %q: %w", key, err)
	}

	if !org.Status.Ready {
		return "", fmt.Errorf("LangfuseOrganization %q is not ready", key)
	}

	if org.Status.OrganizationID == "" {
		return "", fmt.Errorf("LangfuseOrganization %q has no organization ID set", key)
	}

	return org.Status.OrganizationID, nil
}

// syncProject ensures the project exists in Langfuse and returns its ID.
func (r *LangfuseProjectReconciler) syncProject(ctx context.Context, lfClient langfuse.Client, project *v1alpha1.LangfuseProject, orgID string) (string, error) {
	// If we already have a project ID, verify it still exists.
	if project.Status.ProjectID != "" {
		existing, err := lfClient.GetProject(ctx, project.Status.ProjectID)
		if err == nil {
			_ = existing // Project still exists, no name update API in Langfuse.
			return project.Status.ProjectID, nil
		}
		// If not found, fall through to search/create.
		logf.FromContext(ctx).Info("stored project ID not found in Langfuse, will search by name", "projectId", project.Status.ProjectID)
	}

	// Search for the project by name within the organization.
	projects, err := lfClient.ListProjects(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("listing projects: %w", err)
	}
	for _, p := range projects {
		if p.Name == project.Spec.ProjectName {
			return p.ID, nil
		}
	}

	// Project not found. Create it.
	created, err := lfClient.CreateProject(ctx, orgID, project.Spec.ProjectName)
	if err != nil {
		return "", fmt.Errorf("creating project: %w", err)
	}
	logf.FromContext(ctx).Info("created project in Langfuse", "projectId", created.ID, "name", created.Name)

	return created.ID, nil
}

// syncAPIKeys reconciles API key Secrets to match spec.apiKeys.
func (r *LangfuseProjectReconciler) syncAPIKeys(ctx context.Context, lfClient langfuse.Client, project *v1alpha1.LangfuseProject, instance *v1alpha1.LangfuseInstance) ([]v1alpha1.APIKeyStatus, error) {
	log := logf.FromContext(ctx)
	projectID := project.Status.ProjectID
	if projectID == "" {
		return nil, fmt.Errorf("project ID not set, cannot sync API keys")
	}

	// Build the host URL from the instance.
	host := fmt.Sprintf("http://%s-web.%s.svc:3000", instance.Name, instance.Namespace)

	// Fetch current API keys from Langfuse for validation.
	remoteKeys, err := lfClient.ListAPIKeys(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing remote API keys: %w", err)
	}
	remoteKeysByPublicKey := make(map[string]langfuse.APIKey, len(remoteKeys))
	for _, k := range remoteKeys {
		remoteKeysByPublicKey[k.PublicKey] = k
	}

	statuses := make([]v1alpha1.APIKeyStatus, 0, len(project.Spec.APIKeys))

	for _, keySpec := range project.Spec.APIKeys {
		status := v1alpha1.APIKeyStatus{
			Name:       keySpec.Name,
			SecretName: keySpec.SecretName,
		}

		// Check if the K8s Secret already exists.
		existingSecret := &corev1.Secret{}
		secretKey := types.NamespacedName{Name: keySpec.SecretName, Namespace: project.Namespace}
		secretExists := true
		if err := r.Get(ctx, secretKey, existingSecret); err != nil {
			if apierrors.IsNotFound(err) {
				secretExists = false
			} else {
				return nil, fmt.Errorf("getting API key secret %q: %w", secretKey, err)
			}
		}

		if secretExists {
			// Validate that the public key in the Secret is still valid in Langfuse.
			publicKey := string(existingSecret.Data["publicKey"])
			if _, valid := remoteKeysByPublicKey[publicKey]; valid {
				status.Created = true
				statuses = append(statuses, status)
				continue
			}
			// Key is no longer valid in Langfuse. Delete the Secret and recreate.
			log.Info("API key in Secret is no longer valid in Langfuse, recreating",
				"secret", secretKey, "publicKey", publicKey)
			if err := r.Delete(ctx, existingSecret); err != nil && !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("deleting stale API key secret %q: %w", secretKey, err)
			}
		}

		// Create a new API key in Langfuse.
		keyPair, err := lfClient.CreateAPIKey(ctx, projectID, keySpec.Name)
		if err != nil {
			return nil, fmt.Errorf("creating API key %q: %w", keySpec.Name, err)
		}

		// Create the K8s Secret with the key pair.
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      keySpec.SecretName,
				Namespace: project.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "langfuse",
					"app.kubernetes.io/managed-by": "langfuse-operator",
					"app.kubernetes.io/component":  "api-key",
					"langfuse.palena.ai/instance":  project.Spec.InstanceRef.Name,
					"langfuse.palena.ai/project":   project.Name,
				},
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"publicKey": keyPair.PublicKey,
				"secretKey": keyPair.SecretKey,
				"host":      host,
			},
		}

		// Set owner reference so the Secret is garbage collected with the project CR.
		if err := controllerutil.SetControllerReference(project, secret, r.Scheme); err != nil {
			return nil, fmt.Errorf("setting owner reference on API key secret %q: %w", keySpec.SecretName, err)
		}

		if err := r.Create(ctx, secret); err != nil {
			if apierrors.IsAlreadyExists(err) {
				// Race condition: Secret was recreated between our delete and create.
				// This will be reconciled on the next loop.
				log.Info("API key secret already exists after recreation attempt, will retry", "secret", secretKey)
			} else {
				return nil, fmt.Errorf("creating API key secret %q: %w", keySpec.SecretName, err)
			}
		}

		now := metav1.Now()
		status.Created = true
		status.LastRotated = &now
		statuses = append(statuses, status)

		log.Info("created API key and Secret",
			"name", keySpec.Name,
			"secret", keySpec.SecretName,
			"publicKey", keyPair.PublicKey,
		)
	}

	return statuses, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LangfuseProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseProject{}).
		Owns(&corev1.Secret{}).
		Named("langfuseproject").
		Complete(r)
}
