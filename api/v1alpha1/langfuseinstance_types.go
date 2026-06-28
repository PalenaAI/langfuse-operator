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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ─── Image ──────────────────────────────────────────────────────────────────

// ImageSpec defines the container image for Langfuse.
type ImageSpec struct {
	// Repository is the container image repository.
	// +kubebuilder:default="langfuse/langfuse"
	// +optional
	Repository string `json:"repository,omitempty"`
	// Tag is the container image tag.
	Tag string `json:"tag"`
	// PullPolicy defines the image pull policy.
	// +kubebuilder:default="IfNotPresent"
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
	// PullSecrets is a list of references to secrets for pulling the image.
	// +optional
	PullSecrets []corev1.LocalObjectReference `json:"pullSecrets,omitempty"`
}

// WorkerImageSpec overrides the container image for the Langfuse Worker
// component. The Worker runs a different image than the Web component
// (langfuse/langfuse-worker vs langfuse/langfuse); only Repository and Tag are
// configurable here — pull policy and pull secrets are shared with spec.image.
type WorkerImageSpec struct {
	// Repository is the worker container image repository.
	// +kubebuilder:default="langfuse/langfuse-worker"
	// +optional
	Repository string `json:"repository,omitempty"`
	// Tag is the worker container image tag. Defaults to spec.image.tag when
	// left empty, so the Web and Worker stay on the same Langfuse version.
	// +optional
	Tag string `json:"tag,omitempty"`
}

// ─── Web Component ──────────────────────────────────────────────────────────

// WebSpec defines the desired state for the Langfuse Web component.
type WebSpec struct {
	// Replicas is the number of Web pod replicas.
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Autoscaling configures horizontal pod autoscaling.
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
	// Resources defines compute resources for Web pods.
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`
	// PodDisruptionBudget configures the PDB for Web pods.
	// +optional
	PodDisruptionBudget *PDBSpec `json:"podDisruptionBudget,omitempty"`
	// TopologySpreadConstraints configures topology spread.
	// +optional
	TopologySpreadConstraints *TopologySpreadSpec `json:"topologySpreadConstraints,omitempty"`
	// ExtraEnv allows injecting additional environment variables.
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// ExtraVolumeMounts adds additional volume mounts.
	// +optional
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`
	// ExtraVolumes adds additional volumes.
	// +optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`
	// NodeSelector for scheduling Web pods.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations for scheduling Web pods.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// Affinity rules for scheduling Web pods.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// AutoscalingSpec defines HPA configuration.
type AutoscalingSpec struct {
	// Enabled toggles HPA.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// MinReplicas is the lower bound for autoscaling.
	// +kubebuilder:default=1
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// MaxReplicas is the upper bound for autoscaling.
	// +kubebuilder:default=10
	// +optional
	MaxReplicas int32 `json:"maxReplicas,omitempty"`
	// TargetCPUUtilization is the target CPU utilization percentage.
	// +kubebuilder:default=80
	// +optional
	TargetCPUUtilization *int32 `json:"targetCPUUtilization,omitempty"`
	// CustomMetrics defines additional scaling metrics.
	// +optional
	CustomMetrics []CustomMetric `json:"customMetrics,omitempty"`
}

// CustomMetric defines a custom metric for autoscaling.
type CustomMetric struct {
	// Type is the metric type.
	Type string `json:"type"`
	// Threshold is the target value.
	Threshold int64 `json:"threshold"`
}

// PDBSpec defines PodDisruptionBudget configuration.
type PDBSpec struct {
	// Enabled toggles PDB creation.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// MinAvailable is the minimum number of pods that must be available.
	// +optional
	MinAvailable *int32 `json:"minAvailable,omitempty"`
}

// TopologySpreadSpec defines topology spread constraints.
type TopologySpreadSpec struct {
	// Enabled toggles topology spread.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// MaxSkew is the maximum spread skew.
	// +kubebuilder:default=1
	// +optional
	MaxSkew *int32 `json:"maxSkew,omitempty"`
	// TopologyKey is the topology domain.
	// +kubebuilder:default="topology.kubernetes.io/zone"
	// +optional
	TopologyKey string `json:"topologyKey,omitempty"`
}

// ─── Worker Component ───────────────────────────────────────────────────────

// WorkerSpec defines the desired state for the Langfuse Worker component.
type WorkerSpec struct {
	// Replicas is the number of Worker pod replicas.
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Image overrides the container image for the Worker component. Langfuse v3
	// ships the queue consumer as a separate image (langfuse/langfuse-worker);
	// the main langfuse/langfuse image only serves the web app/API and does not
	// drain the ingestion queues. By default the operator uses
	// langfuse/langfuse-worker with the same tag, pull policy, and pull secrets
	// as spec.image. Override only to use a custom registry or tag.
	// +optional
	Image *WorkerImageSpec `json:"image,omitempty"`
	// Autoscaling configures horizontal pod autoscaling.
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
	// Resources defines compute resources for Worker pods.
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`
	// PodDisruptionBudget configures the PDB for Worker pods.
	// +optional
	PodDisruptionBudget *PDBSpec `json:"podDisruptionBudget,omitempty"`
	// Concurrency sets LANGFUSE_WORKER_CONCURRENCY.
	// +kubebuilder:default=10
	// +optional
	Concurrency *int32 `json:"concurrency,omitempty"`
	// ExtraEnv allows injecting additional environment variables.
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// ExtraVolumeMounts adds additional volume mounts to the Worker container.
	// +optional
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`
	// ExtraVolumes adds additional volumes to the Worker pod.
	// +optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`
	// NodeSelector for scheduling Worker pods.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations for scheduling Worker pods.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// Affinity rules for scheduling Worker pods.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// ─── Authentication ─────────────────────────────────────────────────────────

// AuthSpec defines authentication configuration.
type AuthSpec struct {
	// NextAuthSecret references or auto-generates the NEXTAUTH_SECRET.
	// +optional
	NextAuthSecret *SecretValue `json:"nextAuthSecret,omitempty"`
	// NextAuthUrl is the canonical URL for NextAuth (NEXTAUTH_URL).
	NextAuthUrl string `json:"nextAuthUrl"`
	// Salt references or auto-generates the encryption salt.
	// +optional
	Salt *SecretValue `json:"salt,omitempty"`
	// EmailPassword configures email/password authentication.
	// +optional
	EmailPassword *EmailPasswordSpec `json:"emailPassword,omitempty"`
	// OIDC configures OpenID Connect authentication.
	// +optional
	OIDC *OIDCSpec `json:"oidc,omitempty"`
	// InitUser configures an initial admin user.
	// +optional
	InitUser *InitUserSpec `json:"initUser,omitempty"`
	// AdminApiKey references or auto-generates the ADMIN_API_KEY used by the
	// operator to manage organizations and projects via the Langfuse
	// organization-management API. If SecretRef is nil, the operator generates
	// a key and stores it in the auto-generated secret. Required for the
	// LangfuseOrganization and LangfuseProject controllers to function.
	// +optional
	AdminApiKey *SecretValue `json:"adminApiKey,omitempty"`
}

// SecretValue represents a value that can come from a Secret reference.
// If SecretRef is nil and auto-generation is enabled, the operator will generate the value.
type SecretValue struct {
	// SecretRef references an existing Secret key.
	// +optional
	SecretRef *SecretKeyRef `json:"secretRef,omitempty"`
}

// EmailPasswordSpec configures email/password auth.
type EmailPasswordSpec struct {
	// Enabled toggles email/password auth.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// DisableSignup disables new user registration.
	// +optional
	DisableSignup bool `json:"disableSignup,omitempty"`
}

// OIDCSpec configures OpenID Connect authentication via Langfuse's generic
// custom OIDC provider. The operator maps these fields to the upstream
// AUTH_CUSTOM_* environment variables (NextAuth runs on the Web container).
//
// The identity provider must whitelist the callback/redirect URL
// <NEXTAUTH_URL>/api/auth/callback/custom (e.g.
// https://langfuse.example.com/api/auth/callback/custom).
type OIDCSpec struct {
	// Enabled toggles OIDC.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Issuer is the OIDC issuer URL. Maps to AUTH_CUSTOM_ISSUER.
	// +optional
	Issuer string `json:"issuer,omitempty"`
	// ClientId references the OIDC client ID. Maps to AUTH_CUSTOM_CLIENT_ID.
	// +optional
	ClientId *SecretKeyRef `json:"clientId,omitempty"`
	// ClientSecret references the OIDC client secret. Maps to
	// AUTH_CUSTOM_CLIENT_SECRET.
	// +optional
	ClientSecret *SecretKeyRef `json:"clientSecret,omitempty"`
	// Name is the display label for the SSO login button in the Langfuse UI.
	// Maps to AUTH_CUSTOM_NAME; defaults to "SSO" when empty.
	// +optional
	Name string `json:"name,omitempty"`
	// Scope is the list of OAuth scopes requested from the provider. Maps to
	// AUTH_CUSTOM_SCOPE (space-joined); defaults to "openid email profile" when
	// empty.
	// +optional
	Scope []string `json:"scope,omitempty"`
	// SSOEnforcedDomains is a list of email domains that are only allowed to
	// sign in via SSO; email/password sign-in is disabled for these domains.
	// Maps to the upstream AUTH_DOMAINS_WITH_SSO_ENFORCEMENT variable
	// (comma-joined). Note this is a global SSO-enforcement setting, not a
	// per-provider allow-list — upstream Langfuse has no generic
	// AUTH_CUSTOM_ALLOWED_DOMAINS equivalent.
	// +optional
	SSOEnforcedDomains []string `json:"ssoEnforcedDomains,omitempty"`
}

// InitUserSpec configures an initial admin user created on first boot.
type InitUserSpec struct {
	// Enabled toggles init user creation.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Email is the initial user's email.
	// +optional
	Email string `json:"email,omitempty"`
	// Password references the initial user's password.
	// +optional
	Password *SecretKeyRef `json:"password,omitempty"`
	// OrgName is the name of the default organization.
	// +kubebuilder:default="Default"
	// +optional
	OrgName string `json:"orgName,omitempty"`
	// ProjectName is the name of the default project.
	// +kubebuilder:default="Default"
	// +optional
	ProjectName string `json:"projectName,omitempty"`
}

// ─── Datastore TLS ──────────────────────────────────────────────────────────

// TLSSpec configures trust for encrypted (TLS) connections from the Langfuse
// Web and Worker components to their backing datastores.
type TLSSpec struct {
	// TrustedCASecretRef references a Secret (typically cert-manager-issued)
	// holding a CA certificate to trust for outbound TLS. It is mounted into
	// both the Web and Worker pods and exported via NODE_EXTRA_CA_CERTS, which
	// makes the Node.js runtime trust the CA for all outbound TLS — covering
	// ClickHouse HTTPS out of the box. It also serves as the default CA for the
	// Redis and PostgreSQL connections when their own caSecretRef is omitted.
	// +optional
	TrustedCASecretRef *CACertSecretRef `json:"trustedCASecretRef,omitempty"`
}

// CACertSecretRef references a CA certificate within a Secret. cert-manager
// stores the issuing CA under the "ca.crt" key, which is the default.
type CACertSecretRef struct {
	// Name is the name of the Secret.
	Name string `json:"name"`
	// Key is the Secret key holding the PEM-encoded CA certificate.
	// +kubebuilder:default="ca.crt"
	// +optional
	Key string `json:"key,omitempty"`
}

// ClientCertSecretRef references a client certificate/key pair within a Secret
// for mutual TLS. cert-manager stores these under the standard "tls.crt" and
// "tls.key" keys, which are the defaults.
type ClientCertSecretRef struct {
	// Name is the name of the Secret.
	Name string `json:"name"`
	// CertKey is the Secret key holding the PEM-encoded client certificate.
	// +kubebuilder:default="tls.crt"
	// +optional
	CertKey string `json:"certKey,omitempty"`
	// KeyKey is the Secret key holding the PEM-encoded client private key.
	// +kubebuilder:default="tls.key"
	// +optional
	KeyKey string `json:"keyKey,omitempty"`
}

// ─── Secret Management ──────────────────────────────────────────────────────

// SecretManagementSpec configures automatic secret generation and rotation.
type SecretManagementSpec struct {
	// AutoGenerate configures automatic secret generation.
	// +optional
	AutoGenerate *AutoGenerateSpec `json:"autoGenerate,omitempty"`
	// Rotation configures secret rotation detection.
	// +optional
	Rotation *RotationSpec `json:"rotation,omitempty"`
}

// AutoGenerateSpec configures automatic secret generation.
type AutoGenerateSpec struct {
	// Enabled toggles auto-generation of secrets.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// RotationSpec configures secret rotation handling.
type RotationSpec struct {
	// Enabled toggles secret rotation detection.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// CustomMappings defines additional secret-to-component restart mappings.
	// +optional
	CustomMappings []SecretRestartMapping `json:"customMappings,omitempty"`
}

// SecretRestartMapping maps a secret name to components that should restart on change.
type SecretRestartMapping struct {
	// SecretName is the name of the Secret to watch.
	SecretName string `json:"secretName"`
	// RestartComponents lists components to restart (web, worker).
	RestartComponents []string `json:"restartComponents"`
}

// ─── Database (PostgreSQL) ──────────────────────────────────────────────────

// DatabaseSpec defines PostgreSQL configuration.
// Exactly one of CloudNativePG, Managed, or External must be set.
type DatabaseSpec struct {
	// CloudNativePG references an existing CNPG Cluster.
	// +optional
	CloudNativePG *CloudNativePGSpec `json:"cloudnativepg,omitempty"`
	// Managed deploys a PostgreSQL instance managed by the operator.
	// +optional
	Managed *ManagedDatabaseSpec `json:"managed,omitempty"`
	// External references an external PostgreSQL instance.
	// +optional
	External *ExternalDatabaseSpec `json:"external,omitempty"`
	// Migration configures database migration behavior.
	// +optional
	Migration *MigrationSpec `json:"migration,omitempty"`
}

// CloudNativePGSpec references a CloudNativePG Cluster.
type CloudNativePGSpec struct {
	// ClusterRef references the CNPG Cluster.
	ClusterRef ObjectReference `json:"clusterRef"`
	// Database is the database name within the cluster.
	// +kubebuilder:default="langfuse"
	// +optional
	Database string `json:"database,omitempty"`
}

// ManagedDatabaseSpec deploys a managed PostgreSQL instance.
type ManagedDatabaseSpec struct {
	// Instances is the number of PostgreSQL instances.
	// +kubebuilder:default=1
	// +optional
	Instances *int32 `json:"instances,omitempty"`
	// StorageSize is the PVC size for each instance.
	// +kubebuilder:default="10Gi"
	// +optional
	StorageSize string `json:"storageSize,omitempty"`
	// StorageClass is the storage class for PVCs.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
	// Backup configures automated backups.
	// +optional
	Backup *DatabaseBackupSpec `json:"backup,omitempty"`
}

// DatabaseBackupSpec configures database backups.
type DatabaseBackupSpec struct {
	// Enabled toggles automated backups.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Schedule is the cron schedule for backups.
	// +kubebuilder:default="0 2 * * *"
	// +optional
	Schedule string `json:"schedule,omitempty"`
}

// ExternalDatabaseSpec references an external PostgreSQL instance.
type ExternalDatabaseSpec struct {
	// SecretRef references a Secret containing connection details.
	SecretRef SecretKeysRef `json:"secretRef"`
	// TLS configures encrypted connections to the external PostgreSQL instance.
	// +optional
	TLS *DatabaseTLSSpec `json:"tls,omitempty"`
}

// DatabaseTLSSpec configures TLS for the PostgreSQL connection.
//
// Langfuse connects to PostgreSQL through Prisma. Because the connection string
// is supplied via a Secret, the operator does not rewrite it directly; instead
// it sources the base URL into DATABASE_URL_BASE and composes DATABASE_URL as
// "$(DATABASE_URL_BASE)?<tls params>" using Kubernetes env-var interpolation.
// The base URL in the Secret therefore must NOT contain its own query string.
//
// Prisma's PostgreSQL connector uses Prisma-specific parameters (not libpq's):
// "sslmode", "sslaccept" (strict|accept_invalid_certs) and "sslcert" (path to
// the CA certificate — note libpq's "sslrootcert" is NOT supported).
type DatabaseTLSSpec struct {
	// SSLMode selects the verification level, mapped to Prisma parameters:
	//   - "disable"     → sslmode=disable (no TLS)
	//   - "require"     → sslmode=require&sslaccept=accept_invalid_certs (encrypt, no verification)
	//   - "verify-ca"   → sslmode=require&sslaccept=strict (verify the CA chain)
	//   - "verify-full" → sslmode=require&sslaccept=strict (verify CA + hostname)
	// Prisma has no CA-only mode, so "verify-ca" and "verify-full" are
	// equivalent (both enable strict verification).
	// +kubebuilder:validation:Enum=disable;require;verify-ca;verify-full
	// +kubebuilder:default="require"
	// +optional
	SSLMode string `json:"sslMode,omitempty"`
	// CASecretRef references the CA certificate used to verify the PostgreSQL
	// server. When omitted, spec.tls.trustedCASecretRef is used. The CA is
	// mounted into the Web, Worker, and migration pods and referenced as the
	// Prisma "sslcert" parameter.
	// +optional
	CASecretRef *CACertSecretRef `json:"caSecretRef,omitempty"`
}

// MigrationSpec configures database migration behavior.
type MigrationSpec struct {
	// RunOnDeploy toggles running migrations on every deployment.
	// +kubebuilder:default=true
	// +optional
	RunOnDeploy *bool `json:"runOnDeploy,omitempty"`
	// BackgroundMigrations configures background migration handling.
	// +optional
	BackgroundMigrations *BackgroundMigrationSpec `json:"backgroundMigrations,omitempty"`
}

// BackgroundMigrationSpec configures background migration monitoring.
type BackgroundMigrationSpec struct {
	// Enabled toggles background migration monitoring.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Timeout is the maximum duration to wait for background migrations.
	// +kubebuilder:default="3600s"
	// +optional
	Timeout string `json:"timeout,omitempty"`
}

// ─── ClickHouse ─────────────────────────────────────────────────────────────

// ClickHouseSpec defines ClickHouse configuration.
// Exactly one of Managed or External must be set.
type ClickHouseSpec struct {
	// Managed deploys a ClickHouse instance via the ClickHouse Operator.
	// +optional
	Managed *ManagedClickHouseSpec `json:"managed,omitempty"`
	// External references an external ClickHouse instance.
	// +optional
	External *ExternalClickHouseSpec `json:"external,omitempty"`
	// Encryption configures ClickHouse encryption settings.
	// +optional
	Encryption *ClickHouseEncryptionSpec `json:"encryption,omitempty"`
	// Retention configures data retention policies.
	// +optional
	Retention *RetentionSpec `json:"retention,omitempty"`
	// SchemaDrift configures schema drift detection.
	// +optional
	SchemaDrift *SchemaDriftSpec `json:"schemaDrift,omitempty"`
}

// ManagedClickHouseSpec deploys a managed ClickHouse instance.
type ManagedClickHouseSpec struct {
	// Shards is the number of ClickHouse shards.
	// +kubebuilder:default=1
	// +optional
	Shards *int32 `json:"shards,omitempty"`
	// Replicas is the number of ClickHouse replicas per shard.
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// StorageSize is the PVC size.
	// +kubebuilder:default="50Gi"
	// +optional
	StorageSize string `json:"storageSize,omitempty"`
	// StorageClass is the storage class for PVCs.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
	// Resources defines resource presets or custom resources.
	// +optional
	Resources *ClickHouseResourceSpec `json:"resources,omitempty"`
	// Auth references or auto-generates ClickHouse credentials.
	// +optional
	Auth *ClickHouseAuthSpec `json:"auth,omitempty"`
}

// ClickHouseResourceSpec defines ClickHouse resource configuration.
type ClickHouseResourceSpec struct {
	// Preset selects a predefined resource configuration.
	// +kubebuilder:validation:Enum=small;medium;large;custom
	// +optional
	Preset string `json:"preset,omitempty"`
	// Custom defines custom resource requirements (used when preset is "custom").
	// +optional
	Custom *ResourceRequirements `json:"custom,omitempty"`
}

// ClickHouseAuthSpec defines ClickHouse authentication.
type ClickHouseAuthSpec struct {
	// SecretRef references an existing Secret with ClickHouse credentials.
	// +optional
	SecretRef *SecretKeysRef `json:"secretRef,omitempty"`
}

// ExternalClickHouseSpec references an external ClickHouse instance.
type ExternalClickHouseSpec struct {
	// SecretRef references a Secret containing connection details.
	SecretRef SecretKeysRef `json:"secretRef"`
	// TLS configures encrypted connections to the external ClickHouse instance.
	// +optional
	TLS *ClickHouseTLSSpec `json:"tls,omitempty"`
}

// ClickHouseTLSSpec configures TLS for the ClickHouse connection.
//
// Langfuse talks to ClickHouse over two channels: the runtime HTTP client
// (CLICKHOUSE_URL) and the native-protocol migration client
// (CLICKHOUSE_MIGRATION_URL). When Enabled, the operator sets
// CLICKHOUSE_MIGRATION_SSL=true; CA trust for the HTTPS runtime client comes
// from the Node.js trust store via spec.tls.trustedCASecretRef
// (NODE_EXTRA_CA_CERTS) — ClickHouse has no dedicated CA-path variable.
//
// The connection URLs come from the referenced Secret, so they must already use
// the TLS schemes/ports: CLICKHOUSE_URL "https://<host>:8443" and
// CLICKHOUSE_MIGRATION_URL "clickhouse://<host>:9440".
type ClickHouseTLSSpec struct {
	// Enabled toggles TLS for the ClickHouse connection.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// ClickHouseEncryptionSpec configures ClickHouse encryption.
type ClickHouseEncryptionSpec struct {
	// Enabled toggles encryption at rest.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// BlobStorage toggles blob storage encryption.
	// +optional
	BlobStorage bool `json:"blobStorage,omitempty"`
}

// RetentionSpec defines data retention policies for ClickHouse tables.
type RetentionSpec struct {
	// Traces configures retention for trace data.
	// +optional
	Traces *TableRetentionSpec `json:"traces,omitempty"`
	// Observations configures retention for observation data.
	// +optional
	Observations *TableRetentionSpec `json:"observations,omitempty"`
	// Scores configures retention for score data.
	// +optional
	Scores *TableRetentionSpec `json:"scores,omitempty"`
	// StoragePressure configures automatic retention under storage pressure.
	// +optional
	StoragePressure *StoragePressureSpec `json:"storagePressure,omitempty"`
}

// TableRetentionSpec defines TTL for a specific table.
type TableRetentionSpec struct {
	// TTLDays is the number of days to retain data. 0 means infinite.
	// +optional
	TTLDays int32 `json:"ttlDays,omitempty"`
}

// StoragePressureSpec configures behavior under ClickHouse storage pressure.
type StoragePressureSpec struct {
	// Enabled toggles storage pressure monitoring.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// WarningThresholdPercent triggers a warning event.
	// +kubebuilder:default=75
	// +optional
	WarningThresholdPercent int32 `json:"warningThresholdPercent,omitempty"`
	// CriticalThresholdPercent triggers automatic pruning.
	// +kubebuilder:default=90
	// +optional
	CriticalThresholdPercent int32 `json:"criticalThresholdPercent,omitempty"`
	// PruneOldestPartitions enables pruning of oldest partitions.
	// +optional
	PruneOldestPartitions bool `json:"pruneOldestPartitions,omitempty"`
	// MinRetainDays is the minimum retention even under storage pressure.
	// +kubebuilder:default=7
	// +optional
	MinRetainDays int32 `json:"minRetainDays,omitempty"`
}

// SchemaDriftSpec configures ClickHouse schema drift detection.
type SchemaDriftSpec struct {
	// Enabled toggles schema drift detection.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// CheckIntervalMinutes is the interval between drift checks.
	// +kubebuilder:default=60
	// +optional
	CheckIntervalMinutes int32 `json:"checkIntervalMinutes,omitempty"`
	// AutoRepair enables automatic schema repair.
	// +optional
	AutoRepair bool `json:"autoRepair,omitempty"`
}

// ─── Redis / Valkey ─────────────────────────────────────────────────────────

// RedisSpec defines Redis/Valkey configuration.
// Exactly one of Managed or External must be set.
type RedisSpec struct {
	// Managed deploys a Redis instance managed by the operator.
	// +optional
	Managed *ManagedRedisSpec `json:"managed,omitempty"`
	// External references an external Redis instance.
	// +optional
	External *ExternalRedisSpec `json:"external,omitempty"`
}

// ManagedRedisSpec deploys a managed Redis instance.
type ManagedRedisSpec struct {
	// Replicas is the number of Redis replicas.
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// StorageSize is the PVC size.
	// +kubebuilder:default="5Gi"
	// +optional
	StorageSize string `json:"storageSize,omitempty"`
}

// ExternalRedisSpec references an external Redis instance.
type ExternalRedisSpec struct {
	// SecretRef references a Secret containing connection details.
	SecretRef SecretKeysRef `json:"secretRef"`
	// TLS configures encrypted connections to the external Redis instance.
	// +optional
	TLS *RedisTLSSpec `json:"tls,omitempty"`
}

// RedisTLSSpec configures TLS for the Redis/Valkey connection.
//
// Langfuse (via ioredis) reads the CA from REDIS_TLS_CA_PATH directly into the
// client's trust anchors — it does NOT consult the Node.js trust store, so
// NODE_EXTRA_CA_CERTS does not cover Redis. The operator therefore always
// points REDIS_TLS_CA_PATH at the per-connection CA, or at
// spec.tls.trustedCASecretRef when CASecretRef is omitted. With neither set,
// verification falls back to the system trust store (suitable for Redis served
// by a publicly-trusted CA).
type RedisTLSSpec struct {
	// Enabled toggles TLS (REDIS_TLS_ENABLED=true) for both Web and Worker.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// CASecretRef references the CA certificate used to verify the Redis
	// server (REDIS_TLS_CA_PATH). Defaults to spec.tls.trustedCASecretRef.
	// +optional
	CASecretRef *CACertSecretRef `json:"caSecretRef,omitempty"`
	// ClientCertSecretRef references a client certificate/key pair for mutual
	// TLS (REDIS_TLS_CERT_PATH / REDIS_TLS_KEY_PATH).
	// +optional
	ClientCertSecretRef *ClientCertSecretRef `json:"clientCertSecretRef,omitempty"`
	// ServerName overrides the TLS SNI/hostname used to verify the server
	// certificate (REDIS_TLS_SERVERNAME). Useful when connecting via an IP or
	// a name that differs from the certificate's subject.
	// +optional
	ServerName string `json:"serverName,omitempty"`
}

// ─── Blob Storage ───────────────────────────────────────────────────────────

// BlobStorageSpec defines blob storage configuration.
type BlobStorageSpec struct {
	// Provider is the blob storage provider.
	// +kubebuilder:validation:Enum=s3;azure;gcs
	// +optional
	Provider string `json:"provider,omitempty"`
	// S3 configures S3-compatible storage.
	// +optional
	S3 *S3Spec `json:"s3,omitempty"`
	// Azure configures Azure Blob Storage.
	// +optional
	Azure *AzureBlobSpec `json:"azure,omitempty"`
	// GCS configures Google Cloud Storage.
	// +optional
	GCS *GCSSpec `json:"gcs,omitempty"`
}

// S3Spec defines S3-compatible blob storage configuration.
type S3Spec struct {
	// Endpoint is the S3 endpoint URL.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Region is the S3 region.
	// +optional
	Region string `json:"region,omitempty"`
	// Bucket is the S3 bucket name.
	Bucket string `json:"bucket"`
	// ForcePathStyle enables path-style S3 access.
	// +optional
	ForcePathStyle bool `json:"forcePathStyle,omitempty"`
	// Credentials references S3 credentials.
	// +optional
	Credentials *S3CredentialsSpec `json:"credentials,omitempty"`
}

// S3CredentialsSpec references S3 credentials.
type S3CredentialsSpec struct {
	// SecretRef references a Secret containing S3 credentials.
	SecretRef SecretKeysRef `json:"secretRef"`
}

// AzureBlobSpec defines Azure Blob Storage configuration.
//
// Langfuse v3 talks to Azure through the shared LANGFUSE_S3_EVENT_UPLOAD_*
// settings with the LANGFUSE_USE_AZURE_BLOB toggle: the container maps to the
// "bucket", the storage account name to the access key ID, and the storage
// account key to the secret access key. Connection strings are not supported by
// Langfuse — provide the storage account key via Credentials.
type AzureBlobSpec struct {
	// StorageAccountName is the Azure storage account name. Used as the access
	// key ID Langfuse passes to the Azure SDK and to derive the default blob
	// endpoint (https://<account>.blob.core.windows.net).
	StorageAccountName string `json:"storageAccountName"`
	// ContainerName is the blob container name (Langfuse's upload "bucket").
	ContainerName string `json:"containerName"`
	// Endpoint overrides the Azure blob service endpoint. Defaults to
	// https://<storageAccountName>.blob.core.windows.net. Set this for Azure
	// Government, sovereign clouds, or Azurite emulators.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Credentials references the Azure storage account key.
	// +optional
	Credentials *AzureCredentialsSpec `json:"credentials,omitempty"`
}

// AzureCredentialsSpec references Azure credentials.
type AzureCredentialsSpec struct {
	// SecretRef references a Secret containing the Azure storage account key
	// under the "accountKey" key (override the key name via the Keys map).
	// Langfuse does not support Azure connection strings; the account key is
	// required for authentication.
	SecretRef SecretKeysRef `json:"secretRef"`
}

// GCSSpec defines Google Cloud Storage configuration.
type GCSSpec struct {
	// BucketName is the GCS bucket name.
	BucketName string `json:"bucketName"`
	// ProjectId is the GCP project ID.
	// +optional
	ProjectId string `json:"projectId,omitempty"`
	// Credentials references GCS credentials.
	// +optional
	Credentials *GCSCredentialsSpec `json:"credentials,omitempty"`
}

// GCSCredentialsSpec references GCS credentials.
type GCSCredentialsSpec struct {
	// SecretRef references a Secret containing GCS credentials.
	SecretRef SecretKeysRef `json:"secretRef"`
}

// ─── LLM Integration ────────────────────────────────────────────────────────

// LLMSpec configures LLM integration (e.g., for evals).
type LLMSpec struct {
	// APIBase is the LLM API base URL.
	// +optional
	APIBase string `json:"apiBase,omitempty"`
	// APIKey references the LLM API key.
	// +optional
	APIKey *SecretKeyRef `json:"apiKey,omitempty"`
	// Model is the LLM model name.
	// +optional
	Model string `json:"model,omitempty"`
}

// ─── Networking ─────────────────────────────────────────────────────────────

// IngressSpec configures Kubernetes Ingress for the Web component.
type IngressSpec struct {
	// Enabled toggles Ingress creation.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// ClassName is the IngressClass name.
	// +optional
	ClassName string `json:"className,omitempty"`
	// Host is the Ingress hostname.
	// +optional
	Host string `json:"host,omitempty"`
	// Annotations are additional Ingress annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// TLS configures TLS for the Ingress.
	// +optional
	TLS *IngressTLSSpec `json:"tls,omitempty"`
}

// IngressTLSSpec configures TLS for Ingress.
type IngressTLSSpec struct {
	// Enabled toggles TLS.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// SecretName is an existing TLS Secret name.
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// CertManager configures cert-manager integration.
	// +optional
	CertManager *CertManagerSpec `json:"certManager,omitempty"`
}

// CertManagerSpec configures cert-manager integration.
type CertManagerSpec struct {
	// IssuerRef references a cert-manager Issuer or ClusterIssuer.
	IssuerRef CertManagerIssuerRef `json:"issuerRef"`
}

// CertManagerIssuerRef references a cert-manager issuer.
type CertManagerIssuerRef struct {
	// Name of the issuer.
	Name string `json:"name"`
	// Kind of the issuer (Issuer or ClusterIssuer).
	// +kubebuilder:default="ClusterIssuer"
	// +kubebuilder:validation:Enum=Issuer;ClusterIssuer
	// +optional
	Kind string `json:"kind,omitempty"`
}

// RouteSpec configures an OpenShift Route for the Web component.
type RouteSpec struct {
	// Enabled toggles Route creation.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Host is the Route hostname.
	// +optional
	Host string `json:"host,omitempty"`
	// Annotations are additional Route annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GatewayAPISpec configures a Gateway API HTTPRoute for the Web component.
type GatewayAPISpec struct {
	// Enabled toggles HTTPRoute creation.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// GatewayRef references the Gateway this route attaches to.
	GatewayRef GatewayRef `json:"gatewayRef"`
	// Hostname is the HTTP hostname to match.
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// Annotations are additional HTTPRoute annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GatewayRef references a Gateway API Gateway.
type GatewayRef struct {
	// Name of the Gateway.
	Name string `json:"name"`
	// Namespace of the Gateway. Defaults to the HTTPRoute namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// SectionName is the listener name on the Gateway.
	// +optional
	SectionName string `json:"sectionName,omitempty"`
}

// ─── Security ───────────────────────────────────────────────────────────────

// SecuritySpec defines security settings.
type SecuritySpec struct {
	// ReadOnlyRootFilesystem toggles read-only root filesystem.
	// +kubebuilder:default=true
	// +optional
	ReadOnlyRootFilesystem *bool `json:"readOnlyRootFilesystem,omitempty"`
	// RunAsNonRoot toggles non-root execution.
	// +kubebuilder:default=true
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty"`
	// NetworkPolicy configures NetworkPolicy creation.
	// +optional
	NetworkPolicy *NetworkPolicySpec `json:"networkPolicy,omitempty"`
	// Telemetry controls Langfuse's built-in telemetry.
	// +optional
	Telemetry *TelemetrySpec `json:"telemetry,omitempty"`
}

// NetworkPolicySpec configures NetworkPolicy.
type NetworkPolicySpec struct {
	// Enabled toggles NetworkPolicy creation.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// TelemetrySpec configures Langfuse telemetry.
type TelemetrySpec struct {
	// Enabled toggles Langfuse's built-in telemetry.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// ─── Observability ──────────────────────────────────────────────────────────

// ObservabilitySpec defines observability configuration.
type ObservabilitySpec struct {
	// ServiceMonitor configures Prometheus ServiceMonitor.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
	// OTEL configures OpenTelemetry integration.
	// +optional
	OTEL *OTELSpec `json:"otel,omitempty"`
}

// ServiceMonitorSpec configures Prometheus ServiceMonitor.
type ServiceMonitorSpec struct {
	// Enabled toggles ServiceMonitor creation.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Interval is the scrape interval.
	// +kubebuilder:default="30s"
	// +optional
	Interval string `json:"interval,omitempty"`
	// Labels are additional labels for the ServiceMonitor.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// OTELSpec configures OpenTelemetry integration.
type OTELSpec struct {
	// Enabled toggles OTEL.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Endpoint is the OTEL collector endpoint.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Protocol is the OTEL protocol.
	// +kubebuilder:validation:Enum=grpc;http
	// +kubebuilder:default="grpc"
	// +optional
	Protocol string `json:"protocol,omitempty"`
}

// ─── Circuit Breaker ────────────────────────────────────────────────────────

// CircuitBreakerSpec configures dependency circuit breaking.
type CircuitBreakerSpec struct {
	// Enabled toggles circuit breaker.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// ClickHouse configures circuit breaker for ClickHouse.
	// +optional
	ClickHouse *ComponentCircuitBreakerSpec `json:"clickhouse,omitempty"`
	// Redis configures circuit breaker for Redis.
	// +optional
	Redis *ComponentCircuitBreakerSpec `json:"redis,omitempty"`
	// Database configures circuit breaker for PostgreSQL.
	// +optional
	Database *ComponentCircuitBreakerSpec `json:"database,omitempty"`
}

// ComponentCircuitBreakerSpec configures circuit breaker for a single component.
type ComponentCircuitBreakerSpec struct {
	// Action defines what to do when the circuit opens.
	// +kubebuilder:validation:Enum=scaleWorkerToZero;emitCriticalEvent;none
	// +optional
	Action string `json:"action,omitempty"`
	// ProbeIntervalSeconds is the health probe interval.
	// +kubebuilder:default=15
	// +optional
	ProbeIntervalSeconds int32 `json:"probeIntervalSeconds,omitempty"`
	// FailureThreshold is the number of failures before opening the circuit.
	// +kubebuilder:default=3
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
	// RecoveryAction defines what to do when the circuit closes.
	// +kubebuilder:validation:Enum=restoreScale;none
	// +optional
	RecoveryAction string `json:"recoveryAction,omitempty"`
}

// ─── Upgrade Strategy ───────────────────────────────────────────────────────

// UpgradeSpec configures the upgrade strategy.
type UpgradeSpec struct {
	// Strategy is the upgrade strategy.
	// +kubebuilder:validation:Enum=rolling
	// +kubebuilder:default="rolling"
	// +optional
	Strategy string `json:"strategy,omitempty"`
	// PreUpgrade configures pre-upgrade actions.
	// +optional
	PreUpgrade *PreUpgradeSpec `json:"preUpgrade,omitempty"`
	// RollingUpdate configures rolling update parameters.
	// +optional
	RollingUpdate *RollingUpdateSpec `json:"rollingUpdate,omitempty"`
	// PostUpgrade configures post-upgrade actions.
	// +optional
	PostUpgrade *PostUpgradeSpec `json:"postUpgrade,omitempty"`
}

// PreUpgradeSpec configures pre-upgrade actions.
type PreUpgradeSpec struct {
	// RunMigrations toggles running migrations before upgrade.
	// +kubebuilder:default=true
	// +optional
	RunMigrations *bool `json:"runMigrations,omitempty"`
	// BackupDatabase toggles triggering a CNPG backup.
	// +optional
	BackupDatabase bool `json:"backupDatabase,omitempty"`
}

// RollingUpdateSpec configures rolling update parameters.
type RollingUpdateSpec struct {
	// MaxUnavailable is the max number of unavailable pods during update.
	// +kubebuilder:default=0
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
	// MaxSurge is the max number of extra pods during update.
	// +kubebuilder:default=1
	// +optional
	MaxSurge *int32 `json:"maxSurge,omitempty"`
}

// PostUpgradeSpec configures post-upgrade actions.
type PostUpgradeSpec struct {
	// RunBackgroundMigrations toggles running background migrations after upgrade.
	// +kubebuilder:default=true
	// +optional
	RunBackgroundMigrations *bool `json:"runBackgroundMigrations,omitempty"`
	// HealthCheckTimeout is the timeout for post-upgrade health checks.
	// +kubebuilder:default="120s"
	// +optional
	HealthCheckTimeout string `json:"healthCheckTimeout,omitempty"`
	// AutoRollback enables automatic rollback on health check failure.
	// +optional
	AutoRollback bool `json:"autoRollback,omitempty"`
}

// ─── Spec ───────────────────────────────────────────────────────────────────

// LangfuseInstanceSpec defines the desired state of LangfuseInstance.
type LangfuseInstanceSpec struct {
	// Image defines the container image.
	Image ImageSpec `json:"image"`
	// Web configures the Langfuse Web component.
	// +optional
	Web WebSpec `json:"web,omitempty"`
	// Worker configures the Langfuse Worker component.
	// +optional
	Worker WorkerSpec `json:"worker,omitempty"`
	// Auth configures authentication.
	Auth AuthSpec `json:"auth"`
	// TLS configures trust for encrypted connections to Langfuse's datastores
	// (PostgreSQL, ClickHouse, Redis). The trusted CA it references is mounted
	// into both the Web and Worker pods.
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`
	// EELicenseKey references the Langfuse self-hosted Enterprise/Pro license
	// key (LANGFUSE_EE_LICENSE_KEY). Required for the organization-management
	// admin API that powers the LangfuseOrganization and LangfuseProject CRDs;
	// without it those CRDs report a RequiresEELicense condition. Not required
	// for a single LangfuseInstance.
	// +optional
	EELicenseKey *SecretValue `json:"eeLicenseKey,omitempty"`
	// Secrets configures secret management.
	// +optional
	Secrets *SecretManagementSpec `json:"secrets,omitempty"`
	// Database configures PostgreSQL.
	// +optional
	Database *DatabaseSpec `json:"database,omitempty"`
	// ClickHouse configures ClickHouse.
	// +optional
	ClickHouse *ClickHouseSpec `json:"clickhouse,omitempty"`
	// Redis configures Redis/Valkey.
	// +optional
	Redis *RedisSpec `json:"redis,omitempty"`
	// BlobStorage configures blob storage.
	// +optional
	BlobStorage *BlobStorageSpec `json:"blobStorage,omitempty"`
	// LLM configures LLM integration.
	// +optional
	LLM *LLMSpec `json:"llm,omitempty"`
	// Ingress configures Kubernetes Ingress.
	// +optional
	Ingress *IngressSpec `json:"ingress,omitempty"`
	// Route configures OpenShift Route.
	// +optional
	Route *RouteSpec `json:"route,omitempty"`
	// GatewayAPI configures Gateway API HTTPRoute.
	// +optional
	GatewayAPI *GatewayAPISpec `json:"gatewayAPI,omitempty"`
	// Security configures security settings.
	// +optional
	Security *SecuritySpec `json:"security,omitempty"`
	// Observability configures monitoring and tracing.
	// +optional
	Observability *ObservabilitySpec `json:"observability,omitempty"`
	// CircuitBreaker configures dependency circuit breaking.
	// +optional
	CircuitBreaker *CircuitBreakerSpec `json:"circuitBreaker,omitempty"`
	// Upgrade configures the upgrade strategy.
	// +optional
	Upgrade *UpgradeSpec `json:"upgrade,omitempty"`
}

// ─── Status ─────────────────────────────────────────────────────────────────

// LangfuseInstanceStatus defines the observed state of LangfuseInstance.
type LangfuseInstanceStatus struct {
	// Ready indicates whether the instance is fully operational.
	Ready bool `json:"ready,omitempty"`
	// Phase is the current lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;Migrating;Running;Degraded;Error
	// +optional
	Phase string `json:"phase,omitempty"`
	// Web reports the state of the Web component.
	// +optional
	Web *ComponentStatus `json:"web,omitempty"`
	// Worker reports the state of the Worker component.
	// +optional
	Worker *WorkerComponentStatus `json:"worker,omitempty"`
	// Database reports the state of the database.
	// +optional
	Database *DatabaseStatus `json:"database,omitempty"`
	// ClickHouse reports the state of ClickHouse.
	// +optional
	ClickHouse *ClickHouseStatus `json:"clickhouse,omitempty"`
	// Redis reports the state of Redis.
	// +optional
	Redis *ConnectionStatus `json:"redis,omitempty"`
	// BlobStorage reports the state of blob storage.
	// +optional
	BlobStorage *BlobStorageStatus `json:"blobStorage,omitempty"`
	// Secrets reports the state of secret management.
	// +optional
	Secrets *SecretsStatus `json:"secrets,omitempty"`
	// Version is the currently running Langfuse version.
	// +optional
	Version string `json:"version,omitempty"`
	// PublicUrl is the public URL of the Langfuse instance.
	// +optional
	PublicUrl string `json:"publicUrl,omitempty"`
	// Organizations is the count of managed organizations.
	// +optional
	Organizations int32 `json:"organizations,omitempty"`
	// Projects is the count of managed projects.
	// +optional
	Projects int32 `json:"projects,omitempty"`
	// Conditions represent the latest observations of the instance's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ComponentStatus reports the state of a deployment component.
type ComponentStatus struct {
	// Replicas is the desired number of replicas.
	Replicas int32 `json:"replicas,omitempty"`
	// ReadyReplicas is the number of ready replicas.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// Endpoint is the internal service endpoint.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// WorkerComponentStatus extends ComponentStatus with worker-specific fields.
type WorkerComponentStatus struct {
	ComponentStatus `json:",inline"`
	// QueueDepth is the current worker queue depth.
	// +optional
	QueueDepth int64 `json:"queueDepth,omitempty"`
	// CircuitBreakerActive indicates if the circuit breaker is tripped.
	// +optional
	CircuitBreakerActive bool `json:"circuitBreakerActive,omitempty"`
	// CircuitBreakerReason explains why the circuit breaker is active.
	// +optional
	CircuitBreakerReason string `json:"circuitBreakerReason,omitempty"`
}

// DatabaseStatus reports the state of the PostgreSQL database.
type DatabaseStatus struct {
	// Connected indicates if the database is reachable.
	Connected bool `json:"connected,omitempty"`
	// MigrationVersion is the current migration version.
	// +optional
	MigrationVersion string `json:"migrationVersion,omitempty"`
	// BackgroundMigrations reports background migration progress.
	// +optional
	BackgroundMigrations *BackgroundMigrationStatus `json:"backgroundMigrations,omitempty"`
}

// BackgroundMigrationStatus reports background migration progress.
type BackgroundMigrationStatus struct {
	Pending   int32 `json:"pending,omitempty"`
	Running   int32 `json:"running,omitempty"`
	Completed int32 `json:"completed,omitempty"`
}

// ClickHouseStatus reports the state of ClickHouse.
type ClickHouseStatus struct {
	// Connected indicates if ClickHouse is reachable.
	Connected bool `json:"connected,omitempty"`
	// StorageUsed is the current storage consumption.
	// +optional
	StorageUsed string `json:"storageUsed,omitempty"`
	// StorageTotal is the total available storage.
	// +optional
	StorageTotal string `json:"storageTotal,omitempty"`
	// SchemaDrift indicates if schema drift was detected.
	// +optional
	SchemaDrift bool `json:"schemaDrift,omitempty"`
	// RetentionApplied indicates if retention policies are active.
	// +optional
	RetentionApplied bool `json:"retentionApplied,omitempty"`
}

// ConnectionStatus reports basic connection state.
type ConnectionStatus struct {
	// Connected indicates if the service is reachable.
	Connected bool `json:"connected,omitempty"`
}

// BlobStorageStatus reports the state of blob storage.
type BlobStorageStatus struct {
	// Connected indicates if blob storage is reachable.
	Connected bool `json:"connected,omitempty"`
	// Provider is the blob storage provider in use.
	// +optional
	Provider string `json:"provider,omitempty"`
}

// SecretsStatus reports the state of secret management.
type SecretsStatus struct {
	// AutoGenerated indicates if secrets were auto-generated.
	AutoGenerated bool `json:"autoGenerated,omitempty"`
	// ManagedSecretName is the name of the generated secrets Secret.
	// +optional
	ManagedSecretName string `json:"managedSecretName,omitempty"`
	// LastRotationCheck is the last time rotation was checked.
	// +optional
	LastRotationCheck *metav1.Time `json:"lastRotationCheck,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LangfuseInstance is the Schema for the langfuseinstances API.
type LangfuseInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LangfuseInstanceSpec   `json:"spec,omitempty"`
	Status LangfuseInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LangfuseInstanceList contains a list of LangfuseInstance.
type LangfuseInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LangfuseInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LangfuseInstance{}, &LangfuseInstanceList{})
}
