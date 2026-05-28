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

// dbWaitImage is the minimal image used by the migration Job's init container
// to poll backing-store TCP ports. busybox ships `nc`, which is all we need.
const dbWaitImage = "busybox:1.36"

// MigrationJobName returns the name for the migration Job.
func MigrationJobName(instance *v1alpha1.LangfuseInstance) string {
	return instance.Name + "-migrate"
}

// waitForStoresScript blocks until PostgreSQL (and, when configured, ClickHouse)
// accept TCP connections. The Langfuse image entrypoint runs `prisma migrate
// deploy` and the ClickHouse migrations immediately and exits non-zero if a
// store is unreachable; without this gate the migration Job races managed /
// CNPG Postgres startup and burns through its backoff limit before the
// database is accepting connections.
//
// Host/port are parsed from DATABASE_URL (postgres://user:pass@host:port/db)
// and CLICKHOUSE_URL (http://host:port) using POSIX parameter expansion so the
// script runs under busybox sh. `##*@` strips credentials (handles passwords
// containing '@'); the path and query string are then trimmed.
const waitForStoresScript = `set -eu
wait_tcp() {
  host="$1"; port="$2"; label="$3"
  echo "waiting for ${label} at ${host}:${port}"
  i=0
  until nc -z "$host" "$port" 2>/dev/null; do
    i=$((i+1))
    if [ "$i" -ge 150 ]; then
      echo "timed out after 5m waiting for ${label} at ${host}:${port}" >&2
      exit 1
    fi
    sleep 2
  done
  echo "${label} is reachable"
}

parse_hostport() {
  # $1 = URL, $2 = default port. Echoes "host port".
  rest="${1#*://}"      # strip scheme
  rest="${rest##*@}"    # strip credentials (up to last @)
  hostport="${rest%%/*}" # strip /path
  hostport="${hostport%%\?*}" # strip ?query
  h="${hostport%%:*}"
  p="${hostport##*:}"
  if [ "$p" = "$h" ] || [ -z "$p" ]; then p="$2"; fi
  echo "$h $p"
}

if [ -n "${DATABASE_URL:-}" ]; then
  set -- $(parse_hostport "$DATABASE_URL" 5432)
  wait_tcp "$1" "$2" PostgreSQL
fi

if [ -n "${CLICKHOUSE_URL:-}" ]; then
  set -- $(parse_hostport "$CLICKHOUSE_URL" 8123)
  wait_tcp "$1" "$2" ClickHouse
fi
`

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
					InitContainers: []corev1.Container{
						{
							// Block migrations until the backing stores accept
							// connections, so the Job doesn't fail-fast and
							// exhaust its backoff limit while Postgres/ClickHouse
							// are still starting.
							Name:    "wait-for-stores",
							Image:   dbWaitImage,
							Command: []string{"sh", "-c", waitForStoresScript},
							Env:     config.CommonEnv,
						},
					},
					Containers: []corev1.Container{
						{
							// Reuse the image's own ENTRYPOINT (which runs Postgres + ClickHouse
							// migrations) and pass `true` so the container exits cleanly once
							// migrations are done. This keeps the operator version-agnostic
							// across upstream changes to the migration mechanism.
							Name:  "langfuse-migrate",
							Image: containerImage(instance),
							Args:  []string{"true"},
							Env:   config.CommonEnv,
						},
					},
				},
			},
		},
	}
}

func int32Ptr(v int32) *int32 { return &v }
