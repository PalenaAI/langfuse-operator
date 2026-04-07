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
	organizationFinalizer = "langfuse.palena.ai/organization-cleanup"
	orgResyncPeriod       = 5 * time.Minute
)

// LangfuseOrganizationReconciler reconciles a LangfuseOrganization object
type LangfuseOrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseorganizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseorganizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=langfuse.palena.ai,resources=langfuseorganizations/finalizers,verbs=update

// Reconcile synchronizes the desired LangfuseOrganization state with the Langfuse instance.
func (r *LangfuseOrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the LangfuseOrganization CR.
	org := &v1alpha1.LangfuseOrganization{}
	if err := r.Get(ctx, req.NamespacedName, org); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching LangfuseOrganization: %w", err)
	}

	// 2. Handle deletion.
	if !org.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, org)
	}

	// 3. Ensure finalizer is set.
	if !controllerutil.ContainsFinalizer(org, organizationFinalizer) {
		controllerutil.AddFinalizer(org, organizationFinalizer)
		if err := r.Update(ctx, org); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
	}

	// 4. Resolve the parent LangfuseInstance.
	instance, err := r.resolveInstance(ctx, org)
	if err != nil {
		meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "InstanceNotReady",
			Message:            err.Error(),
			ObservedGeneration: org.Generation,
		})
		org.Status.Ready = false
		if statusErr := r.Status().Update(ctx, org); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// 5. Build the Langfuse API client.
	lfClient, err := buildLangfuseClient(ctx, r.Client, instance)
	if err != nil {
		meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ClientError",
			Message:            err.Error(),
			ObservedGeneration: org.Generation,
		})
		org.Status.Ready = false
		if statusErr := r.Status().Update(ctx, org); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// 6. Sync organization.
	orgID, err := r.syncOrganization(ctx, lfClient, org)
	if err != nil {
		meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "SyncFailed",
			Message:            fmt.Sprintf("Failed to sync organization: %s", err.Error()),
			ObservedGeneration: org.Generation,
		})
		org.Status.Ready = false
		if statusErr := r.Status().Update(ctx, org); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, fmt.Errorf("syncing organization: %w", err)
	}
	org.Status.OrganizationID = orgID

	// 7. Sync members.
	syncedCount, err := r.syncMembers(ctx, lfClient, org)
	if err != nil {
		meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "MemberSyncFailed",
			Message:            fmt.Sprintf("Failed to sync members: %s", err.Error()),
			ObservedGeneration: org.Generation,
		})
		org.Status.Ready = false
		if statusErr := r.Status().Update(ctx, org); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, fmt.Errorf("syncing members: %w", err)
	}
	org.Status.SyncedMembers = syncedCount
	org.Status.MemberCount = len(org.Spec.Members.Users)

	// 8. Count dependent LangfuseProject CRs.
	projectCount, err := r.countDependentProjects(ctx, org)
	if err != nil {
		log.Error(err, "failed to count dependent projects")
	}
	org.Status.ProjectCount = projectCount

	// 9. Set Ready condition.
	meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Synced",
		Message:            "Organization is synced with Langfuse",
		ObservedGeneration: org.Generation,
	})
	org.Status.Ready = true

	if err := r.Status().Update(ctx, org); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	log.Info("reconciled organization",
		"organizationId", orgID,
		"members", org.Status.MemberCount,
		"projects", org.Status.ProjectCount,
	)

	return ctrl.Result{RequeueAfter: orgResyncPeriod}, nil
}

// handleDeletion processes the deletion of a LangfuseOrganization.
func (r *LangfuseOrganizationReconciler) handleDeletion(ctx context.Context, org *v1alpha1.LangfuseOrganization) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(org, organizationFinalizer) {
		return ctrl.Result{}, nil
	}

	// Check for dependent LangfuseProject CRs. Block deletion if any exist.
	projectCount, err := r.countDependentProjects(ctx, org)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("checking dependent projects: %w", err)
	}
	if projectCount > 0 {
		meta.SetStatusCondition(&org.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "DependentProjectsExist",
			Message:            fmt.Sprintf("Cannot delete: %d LangfuseProject(s) still reference this organization", projectCount),
			ObservedGeneration: org.Generation,
		})
		org.Status.Ready = false
		if statusErr := r.Status().Update(ctx, org); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Delete the organization in Langfuse (best effort).
	if org.Status.OrganizationID != "" {
		instance, err := r.resolveInstance(ctx, org)
		if err != nil {
			log.Error(err, "could not resolve instance during deletion, skipping Langfuse cleanup")
		} else {
			lfClient, err := buildLangfuseClient(ctx, r.Client, instance)
			if err != nil {
				log.Error(err, "could not build Langfuse client during deletion, skipping Langfuse cleanup")
			} else {
				if err := lfClient.DeleteOrganization(ctx, org.Status.OrganizationID); err != nil {
					log.Error(err, "failed to delete organization in Langfuse (best effort)", "organizationId", org.Status.OrganizationID)
				} else {
					log.Info("deleted organization in Langfuse", "organizationId", org.Status.OrganizationID)
				}
			}
		}
	}

	// Remove finalizer.
	controllerutil.RemoveFinalizer(org, organizationFinalizer)
	if err := r.Update(ctx, org); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}

	return ctrl.Result{}, nil
}

// resolveInstance fetches the LangfuseInstance referenced by the organization and verifies it is ready.
func (r *LangfuseOrganizationReconciler) resolveInstance(ctx context.Context, org *v1alpha1.LangfuseOrganization) (*v1alpha1.LangfuseInstance, error) {
	ns := org.Spec.InstanceRef.Namespace
	if ns == "" {
		ns = org.Namespace
	}

	instance := &v1alpha1.LangfuseInstance{}
	key := types.NamespacedName{Name: org.Spec.InstanceRef.Name, Namespace: ns}
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

// syncOrganization ensures the organization exists in Langfuse and returns its ID.
func (r *LangfuseOrganizationReconciler) syncOrganization(ctx context.Context, lfClient langfuse.Client, org *v1alpha1.LangfuseOrganization) (string, error) {
	// If we already have an organization ID, verify it still exists.
	if org.Status.OrganizationID != "" {
		existing, err := lfClient.GetOrganization(ctx, org.Status.OrganizationID)
		if err == nil {
			// Organization exists. Update name if changed.
			if existing.Name != org.Spec.DisplayName {
				if _, err := lfClient.UpdateOrganization(ctx, org.Status.OrganizationID, org.Spec.DisplayName); err != nil {
					return "", fmt.Errorf("updating organization name: %w", err)
				}
			}
			return org.Status.OrganizationID, nil
		}
		// If the error is a 404, the org was deleted externally. Fall through to search/create.
		logf.FromContext(ctx).Info("stored organization ID not found in Langfuse, will search by name", "organizationId", org.Status.OrganizationID)
	}

	// Search for the organization by name.
	orgs, err := lfClient.ListOrganizations(ctx)
	if err != nil {
		return "", fmt.Errorf("listing organizations: %w", err)
	}
	for _, o := range orgs {
		if o.Name == org.Spec.DisplayName {
			return o.ID, nil
		}
	}

	// Organization not found. Create it.
	created, err := lfClient.CreateOrganization(ctx, org.Spec.DisplayName)
	if err != nil {
		return "", fmt.Errorf("creating organization: %w", err)
	}
	logf.FromContext(ctx).Info("created organization in Langfuse", "organizationId", created.ID, "name", created.Name)

	return created.ID, nil
}

// syncMembers reconciles the organization membership to match the spec.
func (r *LangfuseOrganizationReconciler) syncMembers(ctx context.Context, lfClient langfuse.Client, org *v1alpha1.LangfuseOrganization) (int, error) {
	if len(org.Spec.Members.Users) == 0 && !org.Spec.Members.ManagedExclusively {
		return 0, nil
	}

	orgID := org.Status.OrganizationID
	if orgID == "" {
		return 0, fmt.Errorf("organization ID not set, cannot sync members")
	}

	// Get current members from Langfuse.
	currentMembers, err := lfClient.ListOrgMembers(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("listing current members: %w", err)
	}

	// Build a lookup of current members by email.
	currentByEmail := make(map[string]langfuse.OrgMember, len(currentMembers))
	for _, m := range currentMembers {
		currentByEmail[m.Email] = m
	}

	// Build a set of desired emails for exclusive management.
	desiredEmails := make(map[string]bool, len(org.Spec.Members.Users))

	synced := 0
	for _, desired := range org.Spec.Members.Users {
		desiredEmails[desired.Email] = true

		existing, found := currentByEmail[desired.Email]
		if found {
			// Member exists. Update role if different.
			if existing.Role != desired.Role {
				if err := lfClient.UpdateOrgMemberRole(ctx, orgID, existing.ID, desired.Role); err != nil {
					return synced, fmt.Errorf("updating role for member %q: %w", desired.Email, err)
				}
			}
			synced++
		} else {
			// Member not found. Add them.
			if _, err := lfClient.AddOrgMember(ctx, orgID, desired.Email, desired.Role); err != nil {
				return synced, fmt.Errorf("adding member %q: %w", desired.Email, err)
			}
			synced++
		}
	}

	// If managed exclusively, remove members not in the desired list.
	if org.Spec.Members.ManagedExclusively {
		for _, m := range currentMembers {
			if !desiredEmails[m.Email] {
				if err := lfClient.RemoveOrgMember(ctx, orgID, m.ID); err != nil {
					return synced, fmt.Errorf("removing unlisted member %q: %w", m.Email, err)
				}
			}
		}
	}

	return synced, nil
}

// countDependentProjects counts LangfuseProject CRs that reference this organization.
func (r *LangfuseOrganizationReconciler) countDependentProjects(ctx context.Context, org *v1alpha1.LangfuseOrganization) (int, error) {
	projectList := &v1alpha1.LangfuseProjectList{}
	if err := r.List(ctx, projectList); err != nil {
		return 0, fmt.Errorf("listing LangfuseProjects: %w", err)
	}

	count := 0
	for _, p := range projectList.Items {
		refNs := p.Spec.OrganizationRef.Namespace
		if refNs == "" {
			refNs = p.Namespace
		}
		if p.Spec.OrganizationRef.Name == org.Name && refNs == org.Namespace {
			count++
		}
	}
	return count, nil
}

// buildLangfuseClient creates a Langfuse API client from an instance's operator credentials.
// This is a shared helper used by both organization and project controllers.
func buildLangfuseClient(ctx context.Context, c client.Client, instance *v1alpha1.LangfuseInstance) (langfuse.Client, error) {
	baseURL := fmt.Sprintf("http://%s-web.%s.svc:3000", instance.Name, instance.Namespace)

	credsSecret := &corev1.Secret{}
	credsKey := types.NamespacedName{
		Name:      instance.Name + "-operator-credentials",
		Namespace: instance.Namespace,
	}
	if err := c.Get(ctx, credsKey, credsSecret); err != nil {
		return nil, fmt.Errorf("getting operator credentials: %w", err)
	}

	publicKey := string(credsSecret.Data["publicKey"])
	secretKey := string(credsSecret.Data["secretKey"])
	if publicKey == "" || secretKey == "" {
		return nil, fmt.Errorf("operator credentials secret %q is missing publicKey or secretKey", credsKey)
	}

	return langfuse.NewClient(baseURL, publicKey, secretKey), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LangfuseOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LangfuseOrganization{}).
		Named("langfuseorganization").
		Complete(r)
}
