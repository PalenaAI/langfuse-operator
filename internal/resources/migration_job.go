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

package resources

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
	"github.com/PalenaAI/langfuse-operator/internal/langfuse"
)

// MigrationJobName returns the name for the migration Job.
func MigrationJobName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-migrate"
}

// BuildMigrationJob constructs the desired Job for running Langfuse database migrations.
// The caller is responsible for setting owner references and calling CreateOrUpdate.
func BuildMigrationJob(instance *v1alpha1.LangfuseInstance, config *langfuse.Config) *batchv1.Job {
	labels := CommonLabels(instance, "migration")

	ttl := int32(3600)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MigrationJobName(instance),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            int32Ptr(3),
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "langfuse-migrate",
							Image:   containerImage(instance),
							Command: []string{"node", "packages/shared/dist/src/db/migrate.cjs"},
							Env:     config.CommonEnv,
						},
					},
				},
			},
		},
	}
}

func int32Ptr(v int32) *int32 { return &v }
