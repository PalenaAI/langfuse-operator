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

package langfuse

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/bitkaio/langfuse-operator/api/v1alpha1"
)

func ptrInt32(v int32) *int32 { return &v }
func ptrBool(v bool) *bool    { return &v }

func minimalInstance() *v1alpha1.LangfuseInstance {
	return &v1alpha1.LangfuseInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1alpha1.LangfuseInstanceSpec{
			Image: v1alpha1.ImageSpec{
				Tag: "3",
			},
			Auth: v1alpha1.AuthSpec{
				NextAuthUrl: "https://langfuse.example.com",
			},
		},
	}
}

// ensure corev1 is used (referenced in import for future use)
var _ corev1.EnvVar

func TestBuildConfig_Minimal(t *testing.T) {
	instance := minimalInstance()
	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	// Should have NEXTAUTH_URL
	found := false
	for _, e := range cfg.CommonEnv {
		if e.Name == "NEXTAUTH_URL" {
			if e.Value != "https://langfuse.example.com" {
				t.Errorf("NEXTAUTH_URL = %q, want %q", e.Value, "https://langfuse.example.com")
			}
			found = true
		}
	}
	if !found {
		t.Error("NEXTAUTH_URL not found in CommonEnv")
	}

	// Should have auto-generated secret refs for NEXTAUTH_SECRET and SALT
	for _, name := range []string{"NEXTAUTH_SECRET", "SALT"} {
		found := false
		for _, e := range cfg.CommonEnv {
			if e.Name == name {
				if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
					t.Errorf("%s should reference a secret", name)
				} else if e.ValueFrom.SecretKeyRef.Name != "test-generated-secrets" {
					t.Errorf("%s secret name = %q, want %q", name, e.ValueFrom.SecretKeyRef.Name, "test-generated-secrets")
				}
				found = true
			}
		}
		if !found {
			t.Errorf("%s not found in CommonEnv", name)
		}
	}

	// Web-specific
	found = false
	for _, e := range cfg.WebEnv {
		if e.Name == "LANGFUSE_WORKER_ENABLED" {
			if e.Value != "false" {
				t.Errorf("web LANGFUSE_WORKER_ENABLED = %q, want %q", e.Value, "false")
			}
			found = true
		}
	}
	if !found {
		t.Error("LANGFUSE_WORKER_ENABLED not found in WebEnv")
	}

	// Worker-specific
	found = false
	for _, e := range cfg.WorkerEnv {
		if e.Name == "LANGFUSE_WORKER_ENABLED" {
			if e.Value != "true" {
				t.Errorf("worker LANGFUSE_WORKER_ENABLED = %q, want %q", e.Value, "true")
			}
			found = true
		}
	}
	if !found {
		t.Error("LANGFUSE_WORKER_ENABLED not found in WorkerEnv")
	}

	// Default concurrency
	found = false
	for _, e := range cfg.WorkerEnv {
		if e.Name == "LANGFUSE_WORKER_CONCURRENCY" {
			if e.Value != "10" {
				t.Errorf("LANGFUSE_WORKER_CONCURRENCY = %q, want %q", e.Value, "10")
			}
			found = true
		}
	}
	if !found {
		t.Error("LANGFUSE_WORKER_CONCURRENCY not found in WorkerEnv")
	}
}

func TestBuildConfig_ExplicitSecretRefs(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Auth.NextAuthSecret = &v1alpha1.SecretValue{
		SecretRef: &v1alpha1.SecretKeyRef{Name: "my-secret", Key: "nas"},
	}
	instance.Spec.Auth.Salt = &v1alpha1.SecretValue{
		SecretRef: &v1alpha1.SecretKeyRef{Name: "my-secret", Key: "salt-key"},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	for _, tc := range []struct {
		envName    string
		secretName string
		secretKey  string
	}{
		{"NEXTAUTH_SECRET", "my-secret", "nas"},
		{"SALT", "my-secret", "salt-key"},
	} {
		found := false
		for _, e := range cfg.CommonEnv {
			if e.Name == tc.envName {
				if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
					t.Errorf("%s should reference a secret", tc.envName)
				} else {
					if e.ValueFrom.SecretKeyRef.Name != tc.secretName {
						t.Errorf("%s secret name = %q, want %q", tc.envName, e.ValueFrom.SecretKeyRef.Name, tc.secretName)
					}
					if e.ValueFrom.SecretKeyRef.Key != tc.secretKey {
						t.Errorf("%s secret key = %q, want %q", tc.envName, e.ValueFrom.SecretKeyRef.Key, tc.secretKey)
					}
				}
				found = true
			}
		}
		if !found {
			t.Errorf("%s not found in CommonEnv", tc.envName)
		}
	}
}

func TestBuildConfig_ExternalDatabase(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Database = &v1alpha1.DatabaseSpec{
		External: &v1alpha1.ExternalDatabaseSpec{
			SecretRef: v1alpha1.SecretKeysRef{
				Name: "db-creds",
				Keys: map[string]string{
					"url":       "database_url",
					"directUrl": "direct_url",
				},
			},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	for _, tc := range []struct {
		envName   string
		secretKey string
	}{
		{"DATABASE_URL", "database_url"},
		{"DIRECT_URL", "direct_url"},
	} {
		found := false
		for _, e := range cfg.CommonEnv {
			if e.Name == tc.envName {
				if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
					t.Errorf("%s should reference a secret", tc.envName)
				} else if e.ValueFrom.SecretKeyRef.Key != tc.secretKey {
					t.Errorf("%s secret key = %q, want %q", tc.envName, e.ValueFrom.SecretKeyRef.Key, tc.secretKey)
				}
				found = true
			}
		}
		if !found {
			t.Errorf("%s not found in CommonEnv", tc.envName)
		}
	}
}

func TestBuildConfig_ExternalRedis(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Redis = &v1alpha1.RedisSpec{
		External: &v1alpha1.ExternalRedisSpec{
			SecretRef: v1alpha1.SecretKeysRef{
				Name: "redis-creds",
				Keys: map[string]string{
					"host":     "redis-host",
					"port":     "redis-port",
					"password": "redis-pass",
				},
			},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	for _, tc := range []struct {
		envName   string
		secretKey string
	}{
		{"REDIS_HOST", "redis-host"},
		{"REDIS_PORT", "redis-port"},
		{"REDIS_AUTH", "redis-pass"},
	} {
		found := false
		for _, e := range cfg.CommonEnv {
			if e.Name == tc.envName {
				if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
					t.Errorf("%s should reference a secret", tc.envName)
				} else if e.ValueFrom.SecretKeyRef.Key != tc.secretKey {
					t.Errorf("%s secret key = %q, want %q", tc.envName, e.ValueFrom.SecretKeyRef.Key, tc.secretKey)
				}
				found = true
			}
		}
		if !found {
			t.Errorf("%s not found in CommonEnv", tc.envName)
		}
	}
}

func TestBuildConfig_WorkerConcurrency(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Worker.Concurrency = ptrInt32(20)

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	found := false
	for _, e := range cfg.WorkerEnv {
		if e.Name == "LANGFUSE_WORKER_CONCURRENCY" {
			if e.Value != "20" {
				t.Errorf("LANGFUSE_WORKER_CONCURRENCY = %q, want %q", e.Value, "20")
			}
			found = true
		}
	}
	if !found {
		t.Error("LANGFUSE_WORKER_CONCURRENCY not found in WorkerEnv")
	}
}

func TestBuildConfig_OIDC(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Auth.OIDC = &v1alpha1.OIDCSpec{
		Enabled: true,
		Issuer:  "https://auth.example.com",
		ClientId: &v1alpha1.SecretKeyRef{
			Name: "oidc-secret",
			Key:  "client-id",
		},
		ClientSecret: &v1alpha1.SecretKeyRef{
			Name: "oidc-secret",
			Key:  "client-secret",
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	expectedVars := map[string]string{
		"AUTH_OIDC_ENABLED": "true",
		"AUTH_OIDC_ISSUER":  "https://auth.example.com",
	}

	for name, want := range expectedVars {
		found := false
		for _, e := range cfg.CommonEnv {
			if e.Name == name {
				if e.Value != want {
					t.Errorf("%s = %q, want %q", name, e.Value, want)
				}
				found = true
			}
		}
		if !found {
			t.Errorf("%s not found in CommonEnv", name)
		}
	}
}

func TestBuildConfig_S3BlobStorage(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.BlobStorage = &v1alpha1.BlobStorageSpec{
		Provider: "s3",
		S3: &v1alpha1.S3Spec{
			Bucket:         "my-bucket",
			Region:         "us-east-1",
			Endpoint:       "https://s3.example.com",
			ForcePathStyle: true,
			Credentials: &v1alpha1.S3CredentialsSpec{
				SecretRef: v1alpha1.SecretKeysRef{
					Name: "s3-creds",
					Keys: map[string]string{
						"accessKeyId":     "access-key",
						"secretAccessKey": "secret-key",
					},
				},
			},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	expectedVars := map[string]string{
		"LANGFUSE_S3_EVENT_UPLOAD_ENABLED":          "true",
		"LANGFUSE_S3_EVENT_UPLOAD_BUCKET":           "my-bucket",
		"LANGFUSE_S3_EVENT_UPLOAD_REGION":           "us-east-1",
		"LANGFUSE_S3_EVENT_UPLOAD_ENDPOINT":         "https://s3.example.com",
		"LANGFUSE_S3_EVENT_UPLOAD_FORCE_PATH_STYLE": "true",
	}

	for name, want := range expectedVars {
		found := false
		for _, e := range cfg.CommonEnv {
			if e.Name == name {
				if e.Value != want {
					t.Errorf("%s = %q, want %q", name, e.Value, want)
				}
				found = true
			}
		}
		if !found {
			t.Errorf("%s not found in CommonEnv", name)
		}
	}
}

func TestBuildConfig_TelemetryDisabled(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Security = &v1alpha1.SecuritySpec{
		Telemetry: &v1alpha1.TelemetrySpec{
			Enabled: ptrBool(false),
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	found := false
	for _, e := range cfg.CommonEnv {
		if e.Name == "TELEMETRY_ENABLED" {
			if e.Value != "false" {
				t.Errorf("TELEMETRY_ENABLED = %q, want %q", e.Value, "false")
			}
			found = true
		}
	}
	if !found {
		t.Error("TELEMETRY_ENABLED not found in CommonEnv")
	}
}

func TestBuildConfig_CNPG(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Database = &v1alpha1.DatabaseSpec{
		CloudNativePG: &v1alpha1.CloudNativePGSpec{
			ClusterRef: v1alpha1.ObjectReference{
				Name: "pg-cluster",
			},
			Database: "langfuse",
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	found := false
	for _, e := range cfg.CommonEnv {
		if e.Name == "DATABASE_URL" {
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
				t.Error("DATABASE_URL should reference a secret")
			} else {
				if e.ValueFrom.SecretKeyRef.Name != "pg-cluster-app" {
					t.Errorf("DATABASE_URL secret name = %q, want %q", e.ValueFrom.SecretKeyRef.Name, "pg-cluster-app")
				}
				if e.ValueFrom.SecretKeyRef.Key != "uri" {
					t.Errorf("DATABASE_URL secret key = %q, want %q", e.ValueFrom.SecretKeyRef.Key, "uri")
				}
			}
			found = true
		}
	}
	if !found {
		t.Error("DATABASE_URL not found in CommonEnv")
	}
}
