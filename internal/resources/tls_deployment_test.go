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
	"testing"

	corev1 "k8s.io/api/core/v1"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// tlsInstance is a LangfuseInstance exercising every datastore TLS path so the
// generated Config carries CA + client-cert volumes and the NODE_EXTRA_CA_CERTS
// / REDIS_TLS_* / CLICKHOUSE_MIGRATION_SSL env vars.
func tlsInstance() *v1alpha1.LangfuseInstance {
	instance := minimalInstance()
	instance.Spec.TLS = &v1alpha1.TLSSpec{
		TrustedCASecretRef: &v1alpha1.CACertSecretRef{Name: "internal-ca-bundle"},
	}
	instance.Spec.Database = &v1alpha1.DatabaseSpec{
		External: &v1alpha1.ExternalDatabaseSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "db-creds", Keys: map[string]string{"url": "database_url"}},
			TLS:       &v1alpha1.DatabaseTLSSpec{SSLMode: "verify-full", CASecretRef: &v1alpha1.CACertSecretRef{Name: "pg-tls"}},
		},
	}
	instance.Spec.Redis = &v1alpha1.RedisSpec{
		External: &v1alpha1.ExternalRedisSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "redis-creds", Keys: map[string]string{"host": "h", "port": "p"}},
			TLS: &v1alpha1.RedisTLSSpec{
				Enabled:             true,
				ClientCertSecretRef: &v1alpha1.ClientCertSecretRef{Name: "redis-tls"},
			},
		},
	}
	return instance
}

func mountPaths(mounts []corev1.VolumeMount) map[string]string {
	m := make(map[string]string, len(mounts))
	for _, vm := range mounts {
		m[vm.Name] = vm.MountPath
	}
	return m
}

func volNames(vols []corev1.Volume) map[string]bool {
	m := make(map[string]bool, len(vols))
	for _, v := range vols {
		m[v.Name] = true
	}
	return m
}

// TestBuildDeployments_TLSAppliesToWebAndWorker is the core guarantee: every
// datastore-TLS volume and mount the operator generates must land on BOTH the
// Web and Worker pods. The Worker does most of the Redis/ClickHouse work, so a
// TLS mount missing there would break ingestion under encryption.
func TestBuildDeployments_TLSAppliesToWebAndWorker(t *testing.T) {
	instance := tlsInstance()
	config := buildConfig(instance)

	web := BuildWebDeployment(instance, config)
	worker := BuildWorkerDeployment(instance, config)

	wantVolumes := []string{"langfuse-trusted-ca", "langfuse-postgres-ca", "langfuse-redis-client"}
	wantMounts := map[string]string{
		"langfuse-trusted-ca":   "/etc/langfuse/tls/trusted-ca",
		"langfuse-postgres-ca":  "/etc/langfuse/tls/postgres-ca",
		"langfuse-redis-client": "/etc/langfuse/tls/redis-client",
	}

	checks := map[string]struct {
		vols   map[string]bool
		mounts map[string]string
	}{
		"web":    {volNames(web.Spec.Template.Spec.Volumes), mountPaths(web.Spec.Template.Spec.Containers[0].VolumeMounts)},
		"worker": {volNames(worker.Spec.Template.Spec.Volumes), mountPaths(worker.Spec.Template.Spec.Containers[0].VolumeMounts)},
	}

	for component, got := range checks {
		for _, v := range wantVolumes {
			if !got.vols[v] {
				t.Errorf("%s pod missing TLS volume %q", component, v)
			}
		}
		for name, path := range wantMounts {
			if got.mounts[name] != path {
				t.Errorf("%s container mount %q = %q, want %q", component, name, got.mounts[name], path)
			}
		}
	}
}

func TestBuildDeployments_TLSEnvAppliesToWebAndWorker(t *testing.T) {
	instance := tlsInstance()
	config := buildConfig(instance)

	web := BuildWebDeployment(instance, config)
	worker := BuildWorkerDeployment(instance, config)

	wantEnv := []string{"NODE_EXTRA_CA_CERTS", "REDIS_TLS_ENABLED", "REDIS_TLS_CERT_PATH", "DATABASE_URL"}

	for component, env := range map[string][]corev1.EnvVar{
		"web":    web.Spec.Template.Spec.Containers[0].Env,
		"worker": worker.Spec.Template.Spec.Containers[0].Env,
	} {
		names := make(map[string]bool, len(env))
		for _, e := range env {
			names[e.Name] = true
		}
		for _, want := range wantEnv {
			if !names[want] {
				t.Errorf("%s container missing env %q", component, want)
			}
		}
	}
}

func TestBuildDeployments_WorkerExtraVolumesParity(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Worker.ExtraVolumes = []corev1.Volume{{Name: "scratch"}}
	instance.Spec.Worker.ExtraVolumeMounts = []corev1.VolumeMount{{Name: "scratch", MountPath: "/scratch"}}
	config := buildConfig(instance)

	worker := BuildWorkerDeployment(instance, config)
	if !volNames(worker.Spec.Template.Spec.Volumes)["scratch"] {
		t.Error("worker extraVolumes not applied")
	}
	if mountPaths(worker.Spec.Template.Spec.Containers[0].VolumeMounts)["scratch"] != "/scratch" {
		t.Error("worker extraVolumeMounts not applied")
	}
}
