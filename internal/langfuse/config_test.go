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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
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

	// Should have auto-generated secret refs for NEXTAUTH_SECRET, SALT, and ADMIN_API_KEY
	for _, name := range []string{"NEXTAUTH_SECRET", "SALT", "ADMIN_API_KEY"} {
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
	instance.Spec.Auth.AdminApiKey = &v1alpha1.SecretValue{
		SecretRef: &v1alpha1.SecretKeyRef{Name: "my-secret", Key: "admin-key"},
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
		{"ADMIN_API_KEY", "my-secret", "admin-key"},
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

func envByName(env []corev1.EnvVar, name string) (corev1.EnvVar, bool) {
	for _, e := range env {
		if e.Name == name {
			return e, true
		}
	}
	return corev1.EnvVar{}, false
}

func TestBuildConfig_EELicenseKey(t *testing.T) {
	t.Run("absent by default (no auto-generation)", func(t *testing.T) {
		cfg, err := BuildConfig(minimalInstance())
		if err != nil {
			t.Fatalf("BuildConfig() error: %v", err)
		}
		if _, ok := envByName(cfg.CommonEnv, "LANGFUSE_EE_LICENSE_KEY"); ok {
			t.Error("LANGFUSE_EE_LICENSE_KEY should not be set when eeLicenseKey is nil")
		}
	})

	t.Run("injected from secretRef when provided", func(t *testing.T) {
		instance := minimalInstance()
		instance.Spec.EELicenseKey = &v1alpha1.SecretValue{
			SecretRef: &v1alpha1.SecretKeyRef{Name: "langfuse-ee-license", Key: "license-key"},
		}
		cfg, err := BuildConfig(instance)
		if err != nil {
			t.Fatalf("BuildConfig() error: %v", err)
		}
		e, ok := envByName(cfg.CommonEnv, "LANGFUSE_EE_LICENSE_KEY")
		if !ok {
			t.Fatal("LANGFUSE_EE_LICENSE_KEY not found in CommonEnv")
		}
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			t.Fatal("LANGFUSE_EE_LICENSE_KEY should reference a secret")
		}
		if e.ValueFrom.SecretKeyRef.Name != "langfuse-ee-license" || e.ValueFrom.SecretKeyRef.Key != "license-key" {
			t.Errorf("got ref %s/%s, want langfuse-ee-license/license-key",
				e.ValueFrom.SecretKeyRef.Name, e.ValueFrom.SecretKeyRef.Key)
		}
	})
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
		Name:    "Acme SSO",
		Scope:   []string{"openid", "email", "profile", "groups"},
		ClientId: &v1alpha1.SecretKeyRef{
			Name: "oidc-secret",
			Key:  "client-id",
		},
		ClientSecret: &v1alpha1.SecretKeyRef{
			Name: "oidc-secret",
			Key:  "client-secret",
		},
		SSOEnforcedDomains: []string{"example.com", "acme.com"},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	// Literal AUTH_CUSTOM_* vars.
	expectedVars := map[string]string{
		"AUTH_CUSTOM_ISSUER":                "https://auth.example.com",
		"AUTH_CUSTOM_NAME":                  "Acme SSO",
		"AUTH_CUSTOM_SCOPE":                 "openid email profile groups",
		"AUTH_DOMAINS_WITH_SSO_ENFORCEMENT": "example.com,acme.com",
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

	// Client id/secret must come from secret refs, not literal values.
	expectedSecretRefs := map[string]struct{ secret, key string }{
		"AUTH_CUSTOM_CLIENT_ID":     {"oidc-secret", "client-id"},
		"AUTH_CUSTOM_CLIENT_SECRET": {"oidc-secret", "client-secret"},
	}
	for name, want := range expectedSecretRefs {
		found := false
		for _, e := range cfg.CommonEnv {
			if e.Name != name {
				continue
			}
			found = true
			if e.Value != "" {
				t.Errorf("%s should be a secretKeyRef, got literal value %q", name, e.Value)
			}
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
				t.Fatalf("%s has no secretKeyRef", name)
			}
			ref := e.ValueFrom.SecretKeyRef
			if ref.Name != want.secret || ref.Key != want.key {
				t.Errorf("%s = secret %q/%q, want %q/%q", name, ref.Name, ref.Key, want.secret, want.key)
			}
		}
		if !found {
			t.Errorf("%s not found in CommonEnv", name)
		}
	}

	// The old AUTH_OIDC_* namespace must no longer be emitted.
	for _, e := range cfg.CommonEnv {
		if strings.HasPrefix(e.Name, "AUTH_OIDC_") {
			t.Errorf("unexpected legacy var %q emitted; expected AUTH_CUSTOM_* only", e.Name)
		}
	}
}

func TestBuildConfig_OIDCDefaults(t *testing.T) {
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

	wantDefaults := map[string]string{
		"AUTH_CUSTOM_NAME":  "SSO",
		"AUTH_CUSTOM_SCOPE": "openid email profile",
	}
	for name, want := range wantDefaults {
		found := false
		for _, e := range cfg.CommonEnv {
			if e.Name == name {
				found = true
				if e.Value != want {
					t.Errorf("%s = %q, want default %q", name, e.Value, want)
				}
			}
		}
		if !found {
			t.Errorf("%s not found in CommonEnv", name)
		}
	}

	// No SSO enforcement var when SSOEnforcedDomains is unset.
	for _, e := range cfg.CommonEnv {
		if e.Name == "AUTH_DOMAINS_WITH_SSO_ENFORCEMENT" {
			t.Errorf("AUTH_DOMAINS_WITH_SSO_ENFORCEMENT should not be set when SSOEnforcedDomains is empty")
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

func TestBuildConfig_AzureBlobStorage(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.BlobStorage = &v1alpha1.BlobStorageSpec{
		Provider: "azure",
		Azure: &v1alpha1.AzureBlobSpec{
			StorageAccountName: "stforgesharedprod",
			ContainerName:      "langfuse",
			Credentials: &v1alpha1.AzureCredentialsSpec{
				SecretRef: v1alpha1.SecretKeysRef{
					Name: "langfuse-secrets",
					Keys: map[string]string{"accountKey": "blobAccountKey"},
				},
			},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	// Langfuse v3 reuses the S3 event-upload namespace for Azure. The bucket
	// (container) is the var whose absence triggered the ZodError.
	plain := map[string]string{
		"LANGFUSE_S3_EVENT_UPLOAD_ENABLED":       "true",
		"LANGFUSE_USE_AZURE_BLOB":                "true",
		"LANGFUSE_S3_EVENT_UPLOAD_BUCKET":        "langfuse",
		"LANGFUSE_S3_EVENT_UPLOAD_ENDPOINT":      "https://stforgesharedprod.blob.core.windows.net",
		"LANGFUSE_S3_EVENT_UPLOAD_ACCESS_KEY_ID": "stforgesharedprod",
	}
	for name, want := range plain {
		e, ok := envByName(cfg.CommonEnv, name)
		if !ok {
			t.Errorf("%s not found in CommonEnv", name)
			continue
		}
		if e.Value != want {
			t.Errorf("%s = %q, want %q", name, e.Value, want)
		}
	}

	// The account key must come from the referenced Secret, not the legacy
	// connection-string env vars (which Langfuse ignores).
	e, ok := envByName(cfg.CommonEnv, "LANGFUSE_S3_EVENT_UPLOAD_SECRET_ACCESS_KEY")
	if !ok {
		t.Fatal("LANGFUSE_S3_EVENT_UPLOAD_SECRET_ACCESS_KEY not found in CommonEnv")
	}
	if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
		t.Fatal("LANGFUSE_S3_EVENT_UPLOAD_SECRET_ACCESS_KEY should source from a Secret")
	}
	if e.ValueFrom.SecretKeyRef.Name != "langfuse-secrets" || e.ValueFrom.SecretKeyRef.Key != "blobAccountKey" {
		t.Errorf("account key ref = %s/%s, want langfuse-secrets/blobAccountKey",
			e.ValueFrom.SecretKeyRef.Name, e.ValueFrom.SecretKeyRef.Key)
	}

	// The old, ignored Azure env vars must not be emitted.
	for _, name := range []string{"LANGFUSE_BLOB_STORAGE_PROVIDER", "LANGFUSE_AZURE_STORAGE_ACCOUNT_NAME", "LANGFUSE_AZURE_CONTAINER_NAME"} {
		if _, ok := envByName(cfg.CommonEnv, name); ok {
			t.Errorf("stale env var %s should no longer be emitted", name)
		}
	}
}

func TestBuildConfig_AzureBlobStorage_EndpointOverride(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.BlobStorage = &v1alpha1.BlobStorageSpec{
		Provider: "azure",
		Azure: &v1alpha1.AzureBlobSpec{
			StorageAccountName: "acme",
			ContainerName:      "c",
			Endpoint:           "https://acme.blob.core.usgovcloudapi.net",
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	e, ok := envByName(cfg.CommonEnv, "LANGFUSE_S3_EVENT_UPLOAD_ENDPOINT")
	if !ok {
		t.Fatal("LANGFUSE_S3_EVENT_UPLOAD_ENDPOINT not found")
	}
	if e.Value != "https://acme.blob.core.usgovcloudapi.net" {
		t.Errorf("endpoint = %q, want override", e.Value)
	}
}

func TestBuildConfig_GCSBlobStorage(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.BlobStorage = &v1alpha1.BlobStorageSpec{
		Provider: "gcs",
		GCS: &v1alpha1.GCSSpec{
			BucketName: "my-gcs-bucket",
			Credentials: &v1alpha1.GCSCredentialsSpec{
				SecretRef: v1alpha1.SecretKeysRef{
					Name: "gcs-creds",
					Keys: map[string]string{"credentials": "service-account.json"},
				},
			},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	plain := map[string]string{
		"LANGFUSE_S3_EVENT_UPLOAD_ENABLED":  "true",
		"LANGFUSE_USE_GOOGLE_CLOUD_STORAGE": "true",
		"LANGFUSE_S3_EVENT_UPLOAD_BUCKET":   "my-gcs-bucket",
	}
	for name, want := range plain {
		e, ok := envByName(cfg.CommonEnv, name)
		if !ok {
			t.Errorf("%s not found in CommonEnv", name)
			continue
		}
		if e.Value != want {
			t.Errorf("%s = %q, want %q", name, e.Value, want)
		}
	}

	e, ok := envByName(cfg.CommonEnv, "LANGFUSE_GOOGLE_CLOUD_STORAGE_CREDENTIALS")
	if !ok {
		t.Fatal("LANGFUSE_GOOGLE_CLOUD_STORAGE_CREDENTIALS not found in CommonEnv")
	}
	if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil ||
		e.ValueFrom.SecretKeyRef.Name != "gcs-creds" || e.ValueFrom.SecretKeyRef.Key != "service-account.json" {
		t.Errorf("gcs credentials ref incorrect: %+v", e.ValueFrom)
	}
}

func TestBuildConfig_GCSBlobStorage_WorkloadIdentity(t *testing.T) {
	// No credentials → rely on ambient ADC / Workload Identity, so no
	// credentials env var should be emitted.
	instance := minimalInstance()
	instance.Spec.BlobStorage = &v1alpha1.BlobStorageSpec{
		Provider: "gcs",
		GCS:      &v1alpha1.GCSSpec{BucketName: "wi-bucket"},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	if _, ok := envByName(cfg.CommonEnv, "LANGFUSE_GOOGLE_CLOUD_STORAGE_CREDENTIALS"); ok {
		t.Error("LANGFUSE_GOOGLE_CLOUD_STORAGE_CREDENTIALS should be omitted when no credentials are provided")
	}
	if e, ok := envByName(cfg.CommonEnv, "LANGFUSE_S3_EVENT_UPLOAD_BUCKET"); !ok || e.Value != "wi-bucket" {
		t.Errorf("LANGFUSE_S3_EVENT_UPLOAD_BUCKET = %q (found=%v), want wi-bucket", e.Value, ok)
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

// volumeMountByName returns the VolumeMount with the given name.
func volumeMountByName(mounts []corev1.VolumeMount, name string) (corev1.VolumeMount, bool) {
	for _, m := range mounts {
		if m.Name == name {
			return m, true
		}
	}
	return corev1.VolumeMount{}, false
}

// volumeByName returns the Volume with the given name.
func volumeByName(vols []corev1.Volume, name string) (corev1.Volume, bool) {
	for _, v := range vols {
		if v.Name == name {
			return v, true
		}
	}
	return corev1.Volume{}, false
}

func TestBuildConfig_TrustedCA(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.TLS = &v1alpha1.TLSSpec{
		TrustedCASecretRef: &v1alpha1.CACertSecretRef{Name: "internal-ca-bundle"},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	e, ok := envByName(cfg.CommonEnv, "NODE_EXTRA_CA_CERTS")
	if !ok {
		t.Fatal("NODE_EXTRA_CA_CERTS not set")
	}
	if e.Value != "/etc/langfuse/tls/trusted-ca/ca.crt" {
		t.Errorf("NODE_EXTRA_CA_CERTS = %q, want mounted ca path", e.Value)
	}

	vol, ok := volumeByName(cfg.Volumes, "langfuse-trusted-ca")
	if !ok {
		t.Fatal("trusted-ca volume not mounted")
	}
	if vol.Secret == nil || vol.Secret.SecretName != "internal-ca-bundle" {
		t.Errorf("trusted-ca volume secret = %+v, want internal-ca-bundle", vol.Secret)
	}
	// The secret key (default ca.crt) must be projected to the fixed ca.crt path.
	if len(vol.Secret.Items) != 1 || vol.Secret.Items[0].Key != "ca.crt" || vol.Secret.Items[0].Path != "ca.crt" {
		t.Errorf("trusted-ca items = %+v, want ca.crt->ca.crt", vol.Secret.Items)
	}
	m, ok := volumeMountByName(cfg.VolumeMounts, "langfuse-trusted-ca")
	if !ok || m.MountPath != "/etc/langfuse/tls/trusted-ca" || !m.ReadOnly {
		t.Errorf("trusted-ca mount = %+v, want read-only at /etc/langfuse/tls/trusted-ca", m)
	}
}

func TestBuildConfig_TrustedCA_CustomKey(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.TLS = &v1alpha1.TLSSpec{
		TrustedCASecretRef: &v1alpha1.CACertSecretRef{Name: "bundle", Key: "tls.crt"},
	}
	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}
	vol, _ := volumeByName(cfg.Volumes, "langfuse-trusted-ca")
	if len(vol.Secret.Items) != 1 || vol.Secret.Items[0].Key != "tls.crt" || vol.Secret.Items[0].Path != "ca.crt" {
		t.Errorf("items = %+v, want tls.crt->ca.crt", vol.Secret.Items)
	}
}

func TestBuildConfig_RedisTLS_EnabledTrustedCAFallback(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.TLS = &v1alpha1.TLSSpec{
		TrustedCASecretRef: &v1alpha1.CACertSecretRef{Name: "bundle"},
	}
	instance.Spec.Redis = &v1alpha1.RedisSpec{
		External: &v1alpha1.ExternalRedisSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "redis-creds", Keys: map[string]string{"host": "h", "port": "p"}},
			TLS:       &v1alpha1.RedisTLSSpec{Enabled: true},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	if e, ok := envByName(cfg.CommonEnv, "REDIS_TLS_ENABLED"); !ok || e.Value != "true" {
		t.Errorf("REDIS_TLS_ENABLED = %q (found=%v), want true", e.Value, ok)
	}
	// With no per-connection CA, Redis must fall back to the trusted-CA path
	// (ioredis ignores NODE_EXTRA_CA_CERTS).
	if e, ok := envByName(cfg.CommonEnv, "REDIS_TLS_CA_PATH"); !ok || e.Value != "/etc/langfuse/tls/trusted-ca/ca.crt" {
		t.Errorf("REDIS_TLS_CA_PATH = %q (found=%v), want trusted-ca path", e.Value, ok)
	}
}

func TestBuildConfig_RedisTLS_PerConnectionCAAndMTLS(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Redis = &v1alpha1.RedisSpec{
		External: &v1alpha1.ExternalRedisSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "redis-creds", Keys: map[string]string{"host": "h", "port": "p"}},
			TLS: &v1alpha1.RedisTLSSpec{
				Enabled:             true,
				CASecretRef:         &v1alpha1.CACertSecretRef{Name: "redis-tls"},
				ClientCertSecretRef: &v1alpha1.ClientCertSecretRef{Name: "redis-tls"},
				ServerName:          "redis.internal",
			},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	wantEnv := map[string]string{
		"REDIS_TLS_ENABLED":    "true",
		"REDIS_TLS_CA_PATH":    "/etc/langfuse/tls/redis-ca/ca.crt",
		"REDIS_TLS_CERT_PATH":  "/etc/langfuse/tls/redis-client/tls.crt",
		"REDIS_TLS_KEY_PATH":   "/etc/langfuse/tls/redis-client/tls.key",
		"REDIS_TLS_SERVERNAME": "redis.internal",
	}
	for name, want := range wantEnv {
		if e, ok := envByName(cfg.CommonEnv, name); !ok || e.Value != want {
			t.Errorf("%s = %q (found=%v), want %q", name, e.Value, ok, want)
		}
	}

	if _, ok := volumeByName(cfg.Volumes, "langfuse-redis-ca"); !ok {
		t.Error("redis-ca volume not mounted")
	}
	cc, ok := volumeByName(cfg.Volumes, "langfuse-redis-client")
	if !ok {
		t.Fatal("redis-client volume not mounted")
	}
	if len(cc.Secret.Items) != 2 {
		t.Errorf("redis-client items = %+v, want tls.crt + tls.key", cc.Secret.Items)
	}
}

func TestBuildConfig_RedisTLS_CAWithoutEnabledErrors(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Redis = &v1alpha1.RedisSpec{
		External: &v1alpha1.ExternalRedisSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "redis-creds", Keys: map[string]string{"host": "h", "port": "p"}},
			TLS: &v1alpha1.RedisTLSSpec{
				Enabled:     false,
				CASecretRef: &v1alpha1.CACertSecretRef{Name: "redis-tls"},
			},
		},
	}

	if _, err := BuildConfig(instance); err == nil {
		t.Fatal("expected error when caSecretRef is set but tls.enabled is false")
	}
}

func TestBuildConfig_ClickHouseTLS(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.ClickHouse = &v1alpha1.ClickHouseSpec{
		External: &v1alpha1.ExternalClickHouseSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "ch-creds", Keys: map[string]string{"url": "url"}},
			TLS:       &v1alpha1.ClickHouseTLSSpec{Enabled: true},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	if e, ok := envByName(cfg.CommonEnv, "CLICKHOUSE_MIGRATION_SSL"); !ok || e.Value != "true" {
		t.Errorf("CLICKHOUSE_MIGRATION_SSL = %q (found=%v), want true", e.Value, ok)
	}
}

func TestBuildConfig_ClickHouseTLS_DisabledByDefault(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.ClickHouse = &v1alpha1.ClickHouseSpec{
		External: &v1alpha1.ExternalClickHouseSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "ch-creds", Keys: map[string]string{"url": "url"}},
		},
	}
	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}
	if _, ok := envByName(cfg.CommonEnv, "CLICKHOUSE_MIGRATION_SSL"); ok {
		t.Error("CLICKHOUSE_MIGRATION_SSL should not be set without tls block")
	}
}

func TestBuildConfig_PostgresTLS_VerifyFull(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Database = &v1alpha1.DatabaseSpec{
		External: &v1alpha1.ExternalDatabaseSpec{
			SecretRef: v1alpha1.SecretKeysRef{
				Name: "db-creds",
				Keys: map[string]string{"url": "database_url", "directUrl": "direct_url"},
			},
			TLS: &v1alpha1.DatabaseTLSSpec{
				SSLMode:     "verify-full",
				CASecretRef: &v1alpha1.CACertSecretRef{Name: "pg-tls"},
			},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	// Base URL sourced from the secret.
	base, ok := envByName(cfg.CommonEnv, "DATABASE_URL_BASE")
	if !ok || base.ValueFrom == nil || base.ValueFrom.SecretKeyRef == nil ||
		base.ValueFrom.SecretKeyRef.Key != "database_url" {
		t.Fatalf("DATABASE_URL_BASE should source from secret key database_url, got %+v", base)
	}

	// Effective URL composed via $(VAR) interpolation with Prisma TLS params.
	wantQuery := "$(DATABASE_URL_BASE)?sslmode=require&sslaccept=strict&sslcert=/etc/langfuse/tls/postgres-ca/ca.crt"
	if e, ok := envByName(cfg.CommonEnv, "DATABASE_URL"); !ok || e.Value != wantQuery {
		t.Errorf("DATABASE_URL = %q, want %q", e.Value, wantQuery)
	}
	// DIRECT_URL gets the same treatment.
	if e, ok := envByName(cfg.CommonEnv, "DIRECT_URL"); !ok ||
		e.Value != "$(DIRECT_URL_BASE)?sslmode=require&sslaccept=strict&sslcert=/etc/langfuse/tls/postgres-ca/ca.crt" {
		t.Errorf("DIRECT_URL = %q, want composed with TLS params", e.Value)
	}

	// DATABASE_URL_BASE must appear before DATABASE_URL so $(VAR) resolves.
	baseIdx, urlIdx := -1, -1
	for i, e := range cfg.CommonEnv {
		switch e.Name {
		case "DATABASE_URL_BASE":
			baseIdx = i
		case "DATABASE_URL":
			urlIdx = i
		}
	}
	if baseIdx == -1 || urlIdx == -1 || baseIdx > urlIdx {
		t.Errorf("DATABASE_URL_BASE (idx %d) must precede DATABASE_URL (idx %d)", baseIdx, urlIdx)
	}

	if _, ok := volumeByName(cfg.Volumes, "langfuse-postgres-ca"); !ok {
		t.Error("postgres-ca volume not mounted")
	}
}

func TestBuildConfig_PostgresTLS_RequireUsesTrustedCA(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.TLS = &v1alpha1.TLSSpec{
		TrustedCASecretRef: &v1alpha1.CACertSecretRef{Name: "bundle"},
	}
	instance.Spec.Database = &v1alpha1.DatabaseSpec{
		External: &v1alpha1.ExternalDatabaseSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "db-creds", Keys: map[string]string{"url": "database_url"}},
			TLS:       &v1alpha1.DatabaseTLSSpec{SSLMode: "require"},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	want := "$(DATABASE_URL_BASE)?sslmode=require&sslaccept=accept_invalid_certs&sslcert=/etc/langfuse/tls/trusted-ca/ca.crt"
	if e, ok := envByName(cfg.CommonEnv, "DATABASE_URL"); !ok || e.Value != want {
		t.Errorf("DATABASE_URL = %q, want %q", e.Value, want)
	}
}

func TestBuildConfig_PostgresTLS_Disable(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Database = &v1alpha1.DatabaseSpec{
		External: &v1alpha1.ExternalDatabaseSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "db-creds", Keys: map[string]string{"url": "database_url"}},
			TLS:       &v1alpha1.DatabaseTLSSpec{SSLMode: "disable"},
		},
	}

	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}
	if e, ok := envByName(cfg.CommonEnv, "DATABASE_URL"); !ok || e.Value != "$(DATABASE_URL_BASE)?sslmode=disable" {
		t.Errorf("DATABASE_URL = %q, want sslmode=disable", e.Value)
	}
}

func TestBuildConfig_ExternalDatabase_NoTLSUnchanged(t *testing.T) {
	// Without a tls block the legacy direct DATABASE_URL secret ref is preserved.
	instance := minimalInstance()
	instance.Spec.Database = &v1alpha1.DatabaseSpec{
		External: &v1alpha1.ExternalDatabaseSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "db-creds", Keys: map[string]string{"url": "database_url"}},
		},
	}
	cfg, err := BuildConfig(instance)
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}
	e, ok := envByName(cfg.CommonEnv, "DATABASE_URL")
	if !ok || e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("DATABASE_URL should be a direct secret ref, got %+v", e)
	}
	if _, ok := envByName(cfg.CommonEnv, "DATABASE_URL_BASE"); ok {
		t.Error("DATABASE_URL_BASE should not be emitted without a tls block")
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

// spec.database.managed never deployed PostgreSQL and pointed DATABASE_URL at a
// Secret key nothing generates, so it could only ever produce pods stuck in
// CreateContainerConfigError. It must now fail loudly at config time instead.
func TestBuildConfig_ManagedDatabaseRejected(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Database = &v1alpha1.DatabaseSpec{
		Managed: &v1alpha1.ManagedDatabaseSpec{},
	}

	_, err := BuildConfig(instance)
	if err == nil {
		t.Fatal("expected spec.database.managed to be rejected")
	}
	for _, want := range []string{"not implemented", "cloudnativepg", "external"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestBuildConfig_CNPGAndExternalStillAccepted(t *testing.T) {
	for name, db := range map[string]*v1alpha1.DatabaseSpec{
		"cloudnativepg": {CloudNativePG: &v1alpha1.CloudNativePGSpec{
			ClusterRef: v1alpha1.ObjectReference{Name: "pg"},
		}},
		"external": {External: &v1alpha1.ExternalDatabaseSpec{
			SecretRef: v1alpha1.SecretKeysRef{Name: "db", Keys: map[string]string{"url": "database_url"}},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			instance := minimalInstance()
			instance.Spec.Database = db
			if _, err := BuildConfig(instance); err != nil {
				t.Errorf("BuildConfig() error: %v", err)
			}
		})
	}
}
