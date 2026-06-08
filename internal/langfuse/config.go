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
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// Config holds the computed environment variables for Web and Worker deployments.
type Config struct {
	CommonEnv []corev1.EnvVar
	WebEnv    []corev1.EnvVar
	WorkerEnv []corev1.EnvVar
}

// BuildConfig computes all Langfuse environment variables from the CRD spec.
func BuildConfig(instance *v1alpha1.LangfuseInstance) (*Config, error) {
	cfg := &Config{}

	// ─── Auth ─────────────────────────────────────────────────
	addAuthEnv(cfg, instance)

	// ─── Database ─────────────────────────────────────────────
	addDatabaseEnv(cfg, instance)

	// ─── ClickHouse ───────────────────────────────────────────
	addClickHouseEnv(cfg, instance)

	// ─── Redis ────────────────────────────────────────────────
	addRedisEnv(cfg, instance)

	// ─── Blob Storage ─────────────────────────────────────────
	addBlobStorageEnv(cfg, instance)

	// ─── LLM ──────────────────────────────────────────────────
	if instance.Spec.LLM != nil {
		if instance.Spec.LLM.APIBase != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("LLM_API_BASE", instance.Spec.LLM.APIBase))
		}
		if instance.Spec.LLM.APIKey != nil {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("LLM_API_KEY",
				instance.Spec.LLM.APIKey.Name, instance.Spec.LLM.APIKey.Key))
		}
		if instance.Spec.LLM.Model != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("LLM_MODEL", instance.Spec.LLM.Model))
		}
	}

	// ─── Telemetry ────────────────────────────────────────────
	if instance.Spec.Security != nil && instance.Spec.Security.Telemetry != nil &&
		instance.Spec.Security.Telemetry.Enabled != nil && !*instance.Spec.Security.Telemetry.Enabled {
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("TELEMETRY_ENABLED", "false"))
	}

	// ─── OTEL ─────────────────────────────────────────────────
	if instance.Spec.Observability != nil && instance.Spec.Observability.OTEL != nil && instance.Spec.Observability.OTEL.Enabled {
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("OTEL_EXPORTER_OTLP_ENDPOINT", instance.Spec.Observability.OTEL.Endpoint))
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("OTEL_EXPORTER_OTLP_PROTOCOL", instance.Spec.Observability.OTEL.Protocol))
	}

	// ─── Web-specific ─────────────────────────────────────────
	cfg.WebEnv = append(cfg.WebEnv, envVar("LANGFUSE_WORKER_ENABLED", "false"))
	cfg.WebEnv = append(cfg.WebEnv, envVar("PORT", "3000"))
	cfg.WebEnv = append(cfg.WebEnv, envVar("HOSTNAME", "0.0.0.0"))
	// Migrations are owned by the dedicated migration Job; disable them in the
	// web/worker entrypoints to avoid Prisma advisory-lock deadlocks when
	// multiple pods start in parallel.
	cfg.WebEnv = append(cfg.WebEnv, envVar("LANGFUSE_AUTO_POSTGRES_MIGRATION_DISABLED", "true"))
	cfg.WebEnv = append(cfg.WebEnv, envVar("LANGFUSE_AUTO_CLICKHOUSE_MIGRATION_DISABLED", "true"))

	// ─── Worker-specific ──────────────────────────────────────
	cfg.WorkerEnv = append(cfg.WorkerEnv, envVar("LANGFUSE_WORKER_ENABLED", "true"))
	concurrency := int32(10)
	if instance.Spec.Worker.Concurrency != nil {
		concurrency = *instance.Spec.Worker.Concurrency
	}
	cfg.WorkerEnv = append(cfg.WorkerEnv, envVar("LANGFUSE_WORKER_CONCURRENCY", strconv.Itoa(int(concurrency))))
	cfg.WorkerEnv = append(cfg.WorkerEnv, envVar("LANGFUSE_AUTO_POSTGRES_MIGRATION_DISABLED", "true"))
	cfg.WorkerEnv = append(cfg.WorkerEnv, envVar("LANGFUSE_AUTO_CLICKHOUSE_MIGRATION_DISABLED", "true"))

	return cfg, nil
}

func addAuthEnv(cfg *Config, instance *v1alpha1.LangfuseInstance) {
	cfg.CommonEnv = append(cfg.CommonEnv, corev1.EnvVar{
		Name:  "NEXTAUTH_URL",
		Value: instance.Spec.Auth.NextAuthUrl,
	})

	if instance.Spec.Auth.NextAuthSecret != nil && instance.Spec.Auth.NextAuthSecret.SecretRef != nil {
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("NEXTAUTH_SECRET",
			instance.Spec.Auth.NextAuthSecret.SecretRef.Name,
			instance.Spec.Auth.NextAuthSecret.SecretRef.Key))
	} else {
		// Auto-generated secret reference
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("NEXTAUTH_SECRET",
			generatedSecretName(instance), "nextauth-secret"))
	}

	if instance.Spec.Auth.Salt != nil && instance.Spec.Auth.Salt.SecretRef != nil {
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("SALT",
			instance.Spec.Auth.Salt.SecretRef.Name,
			instance.Spec.Auth.Salt.SecretRef.Key))
	} else {
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("SALT",
			generatedSecretName(instance), "salt"))
	}

	// ADMIN_API_KEY — used by the operator's organization/project controllers
	// to call the Langfuse organization-management API. Injected into the
	// Langfuse containers so the server accepts the operator's Bearer token.
	if instance.Spec.Auth.AdminApiKey != nil && instance.Spec.Auth.AdminApiKey.SecretRef != nil {
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("ADMIN_API_KEY",
			instance.Spec.Auth.AdminApiKey.SecretRef.Name,
			instance.Spec.Auth.AdminApiKey.SecretRef.Key))
	} else {
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("ADMIN_API_KEY",
			generatedSecretName(instance), "admin-api-key"))
	}

	// LANGFUSE_EE_LICENSE_KEY — unlocks the organization-management admin API
	// (an Enterprise/Pro self-hosted feature). Only injected when provided;
	// it cannot be auto-generated. Without it, Org/Project CRDs are inert.
	if instance.Spec.EELicenseKey != nil && instance.Spec.EELicenseKey.SecretRef != nil {
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("LANGFUSE_EE_LICENSE_KEY",
			instance.Spec.EELicenseKey.SecretRef.Name,
			instance.Spec.EELicenseKey.SecretRef.Key))
	}

	// Email/password auth
	if instance.Spec.Auth.EmailPassword != nil {
		if instance.Spec.Auth.EmailPassword.Enabled != nil && !*instance.Spec.Auth.EmailPassword.Enabled {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("AUTH_DISABLE_USERNAME_PASSWORD", "true"))
		}
		if instance.Spec.Auth.EmailPassword.DisableSignup {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("AUTH_DISABLE_SIGNUP", "true"))
		}
	}

	// OIDC
	if instance.Spec.Auth.OIDC != nil && instance.Spec.Auth.OIDC.Enabled {
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("AUTH_OIDC_ENABLED", "true"))
		if instance.Spec.Auth.OIDC.Issuer != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("AUTH_OIDC_ISSUER", instance.Spec.Auth.OIDC.Issuer))
		}
		if instance.Spec.Auth.OIDC.ClientId != nil {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("AUTH_OIDC_CLIENT_ID",
				instance.Spec.Auth.OIDC.ClientId.Name, instance.Spec.Auth.OIDC.ClientId.Key))
		}
		if instance.Spec.Auth.OIDC.ClientSecret != nil {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("AUTH_OIDC_CLIENT_SECRET",
				instance.Spec.Auth.OIDC.ClientSecret.Name, instance.Spec.Auth.OIDC.ClientSecret.Key))
		}
	}

	// Init user
	if instance.Spec.Auth.InitUser != nil && instance.Spec.Auth.InitUser.Enabled {
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_INIT_USER_EMAIL", instance.Spec.Auth.InitUser.Email))
		if instance.Spec.Auth.InitUser.Password != nil {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("LANGFUSE_INIT_USER_PASSWORD",
				instance.Spec.Auth.InitUser.Password.Name, instance.Spec.Auth.InitUser.Password.Key))
		}
		if instance.Spec.Auth.InitUser.OrgName != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_INIT_ORG_NAME", instance.Spec.Auth.InitUser.OrgName))
		}
		if instance.Spec.Auth.InitUser.ProjectName != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_INIT_PROJECT_NAME", instance.Spec.Auth.InitUser.ProjectName))
		}
	}
}

func addDatabaseEnv(cfg *Config, instance *v1alpha1.LangfuseInstance) {
	if instance.Spec.Database == nil {
		return
	}

	db := instance.Spec.Database
	switch {
	case db.CloudNativePG != nil:
		// CNPG stores credentials in <cluster>-app secret with keys: uri, host, port, dbname, user, password
		clusterName := db.CloudNativePG.ClusterRef.Name
		secretName := clusterName + "-app"
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("DATABASE_URL", secretName, "uri"))
	case db.Managed != nil:
		// Managed DB — the operator will create the secret in Phase 2
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("DATABASE_URL",
			generatedSecretName(instance), "database-url"))
	case db.External != nil:
		urlKey := db.External.SecretRef.Keys["url"]
		if urlKey == "" {
			urlKey = "database_url"
		}
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("DATABASE_URL",
			db.External.SecretRef.Name, urlKey))
		// Direct URL for connection pooling bypass
		if directKey, ok := db.External.SecretRef.Keys["directUrl"]; ok && directKey != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("DIRECT_URL",
				db.External.SecretRef.Name, directKey))
		}
	}
}

func addClickHouseEnv(cfg *Config, instance *v1alpha1.LangfuseInstance) {
	if instance.Spec.ClickHouse == nil {
		return
	}

	ch := instance.Spec.ClickHouse

	// Default to single-node mode; clustering requires ZooKeeper/Keeper
	cfg.CommonEnv = append(cfg.CommonEnv, envVar("CLICKHOUSE_CLUSTER_ENABLED", "false"))

	switch {
	case ch.Managed != nil:
		// Managed ClickHouse — credentials from generated or referenced secret
		if ch.Managed.Auth != nil && ch.Managed.Auth.SecretRef != nil {
			usernameKey := ch.Managed.Auth.SecretRef.Keys["username"]
			if usernameKey == "" {
				usernameKey = "username"
			}
			passwordKey := ch.Managed.Auth.SecretRef.Keys["password"]
			if passwordKey == "" {
				passwordKey = "password"
			}
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("CLICKHOUSE_USER",
				ch.Managed.Auth.SecretRef.Name, usernameKey))
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("CLICKHOUSE_PASSWORD",
				ch.Managed.Auth.SecretRef.Name, passwordKey))
		} else {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("CLICKHOUSE_USER",
				generatedSecretName(instance), "clickhouse-username"))
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("CLICKHOUSE_PASSWORD",
				generatedSecretName(instance), "clickhouse-password"))
		}
		// The URL will be set in Phase 2 when managed ClickHouse is deployed
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("CLICKHOUSE_URL",
			fmt.Sprintf("http://%s-clickhouse:8123", instance.Name)))
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("CLICKHOUSE_MIGRATION_URL",
			fmt.Sprintf("clickhouse://%s-clickhouse:9000", instance.Name)))
	case ch.External != nil:
		urlKey := ch.External.SecretRef.Keys["url"]
		if urlKey == "" {
			urlKey = "url"
		}
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("CLICKHOUSE_URL",
			ch.External.SecretRef.Name, urlKey))
		if migrationKey, ok := ch.External.SecretRef.Keys["migrationUrl"]; ok && migrationKey != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("CLICKHOUSE_MIGRATION_URL",
				ch.External.SecretRef.Name, migrationKey))
		}
		if usernameKey, ok := ch.External.SecretRef.Keys["username"]; ok && usernameKey != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("CLICKHOUSE_USER",
				ch.External.SecretRef.Name, usernameKey))
		}
		if passwordKey, ok := ch.External.SecretRef.Keys["password"]; ok && passwordKey != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("CLICKHOUSE_PASSWORD",
				ch.External.SecretRef.Name, passwordKey))
		}
	}
}

func addRedisEnv(cfg *Config, instance *v1alpha1.LangfuseInstance) {
	if instance.Spec.Redis == nil {
		return
	}

	r := instance.Spec.Redis
	switch {
	case r.Managed != nil:
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("REDIS_HOST",
			fmt.Sprintf("%s-redis", instance.Name)))
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("REDIS_PORT", "6379"))
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("REDIS_AUTH",
			generatedSecretName(instance), "redis-password"))
	case r.External != nil:
		hostKey := r.External.SecretRef.Keys["host"]
		if hostKey == "" {
			hostKey = "host"
		}
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("REDIS_HOST",
			r.External.SecretRef.Name, hostKey))
		portKey := r.External.SecretRef.Keys["port"]
		if portKey == "" {
			portKey = "port"
		}
		cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("REDIS_PORT",
			r.External.SecretRef.Name, portKey))
		if passwordKey, ok := r.External.SecretRef.Keys["password"]; ok && passwordKey != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("REDIS_AUTH",
				r.External.SecretRef.Name, passwordKey))
		}
		if tlsKey, ok := r.External.SecretRef.Keys["tls"]; ok && tlsKey != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envFromSecret("REDIS_TLS_ENABLED",
				r.External.SecretRef.Name, tlsKey))
		}
	}
}

func addBlobStorageEnv(cfg *Config, instance *v1alpha1.LangfuseInstance) {
	if instance.Spec.BlobStorage == nil {
		return
	}

	bs := instance.Spec.BlobStorage
	switch bs.Provider {
	case "s3":
		if bs.S3 == nil {
			return
		}
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_ENABLED", "true"))
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_BUCKET", bs.S3.Bucket))
		if bs.S3.Region != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_REGION", bs.S3.Region))
		}
		if bs.S3.Endpoint != "" {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_ENDPOINT", bs.S3.Endpoint))
		}
		if bs.S3.ForcePathStyle {
			cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_FORCE_PATH_STYLE", "true"))
		}
		if bs.S3.Credentials != nil {
			accessKeyID := bs.S3.Credentials.SecretRef.Keys["accessKeyId"]
			if accessKeyID == "" {
				accessKeyID = "access_key"
			}
			secretAccessKey := bs.S3.Credentials.SecretRef.Keys["secretAccessKey"]
			if secretAccessKey == "" {
				secretAccessKey = "secret_key"
			}
			cfg.CommonEnv = append(cfg.CommonEnv,
				envFromSecret("LANGFUSE_S3_EVENT_UPLOAD_ACCESS_KEY_ID",
					bs.S3.Credentials.SecretRef.Name, accessKeyID))
			cfg.CommonEnv = append(cfg.CommonEnv,
				envFromSecret("LANGFUSE_S3_EVENT_UPLOAD_SECRET_ACCESS_KEY",
					bs.S3.Credentials.SecretRef.Name, secretAccessKey))
		}
	case "azure":
		if bs.Azure == nil {
			return
		}
		// Langfuse v3 has no native Azure env namespace; it reuses the S3 event
		// upload settings and switches the storage client via LANGFUSE_USE_AZURE_BLOB.
		// The container is the "bucket", the storage account name is the access
		// key ID, and the account key is the secret access key.
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_ENABLED", "true"))
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_USE_AZURE_BLOB", "true"))
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_BUCKET", bs.Azure.ContainerName))
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_ENDPOINT", azureBlobEndpoint(bs.Azure)))
		// The storage account name is not sensitive; pass it as a plain value.
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_ACCESS_KEY_ID", bs.Azure.StorageAccountName))
		if bs.Azure.Credentials != nil {
			accountKey := bs.Azure.Credentials.SecretRef.Keys["accountKey"]
			if accountKey == "" {
				accountKey = "accountKey"
			}
			cfg.CommonEnv = append(cfg.CommonEnv,
				envFromSecret("LANGFUSE_S3_EVENT_UPLOAD_SECRET_ACCESS_KEY",
					bs.Azure.Credentials.SecretRef.Name, accountKey))
		}
	case "gcs":
		if bs.GCS == nil {
			return
		}
		// GCS, like Azure, rides on the S3 event upload settings; the bucket is
		// shared and the client is switched via LANGFUSE_USE_GOOGLE_CLOUD_STORAGE.
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_ENABLED", "true"))
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_USE_GOOGLE_CLOUD_STORAGE", "true"))
		cfg.CommonEnv = append(cfg.CommonEnv, envVar("LANGFUSE_S3_EVENT_UPLOAD_BUCKET", bs.GCS.BucketName))
		if bs.GCS.Credentials != nil {
			credentialsKey := bs.GCS.Credentials.SecretRef.Keys["credentials"]
			if credentialsKey == "" {
				credentialsKey = "credentials"
			}
			cfg.CommonEnv = append(cfg.CommonEnv,
				envFromSecret("LANGFUSE_GOOGLE_CLOUD_STORAGE_CREDENTIALS",
					bs.GCS.Credentials.SecretRef.Name, credentialsKey))
		}
		// When Credentials is nil, Langfuse falls back to ambient credentials
		// (GKE Workload Identity / Application Default Credentials).
	}
}

// azureBlobEndpoint returns the configured Azure blob endpoint or the default
// derived from the storage account name.
func azureBlobEndpoint(azure *v1alpha1.AzureBlobSpec) string {
	if azure.Endpoint != "" {
		return azure.Endpoint
	}
	return fmt.Sprintf("https://%s.blob.core.windows.net", azure.StorageAccountName)
}

// GeneratedSecretName returns the name of the auto-generated secrets Secret.
func GeneratedSecretName(instance *v1alpha1.LangfuseInstance) string {
	return generatedSecretName(instance)
}

func generatedSecretName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-generated-secrets"
}

func envVar(name, value string) corev1.EnvVar {
	return corev1.EnvVar{Name: name, Value: value}
}

func envFromSecret(envName, secretName, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  key,
			},
		},
	}
}
