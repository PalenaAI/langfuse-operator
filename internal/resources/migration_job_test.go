/*
Copyright 2026 bitkaio LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package resources

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
	"github.com/PalenaAI/langfuse-operator/internal/langfuse"
)

func TestBuildMigrationJob_WaitsForStores(t *testing.T) {
	instance := &v1alpha1.LangfuseInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "lf", Namespace: "ns"},
	}
	cfg := &langfuse.Config{
		CommonEnv: []corev1.EnvVar{
			{Name: "DATABASE_URL", Value: "postgres://u:p@pg:5432/db"},
			{Name: "CLICKHOUSE_URL", Value: "http://ch:8123"},
		},
	}

	job := BuildMigrationJob(instance, cfg)

	// Exactly one init container that gates on the stores.
	inits := job.Spec.Template.Spec.InitContainers
	if len(inits) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(inits))
	}
	wait := inits[0]
	if wait.Name != "wait-for-stores" {
		t.Errorf("init container name = %q, want wait-for-stores", wait.Name)
	}
	if wait.Image != dbWaitImage {
		t.Errorf("init container image = %q, want %q", wait.Image, dbWaitImage)
	}
	// The init container must see the same connection env as the migration
	// container, otherwise it can't resolve the host/port to poll.
	if len(wait.Env) != len(cfg.CommonEnv) {
		t.Errorf("init container env count = %d, want %d", len(wait.Env), len(cfg.CommonEnv))
	}
	if len(wait.Command) != 3 || wait.Command[0] != "sh" || wait.Command[1] != "-c" {
		t.Fatalf("unexpected init command: %v", wait.Command)
	}
	script := wait.Command[2]
	for _, want := range []string{"DATABASE_URL", "CLICKHOUSE_URL", "nc -z", "PostgreSQL", "ClickHouse"} {
		if !strings.Contains(script, want) {
			t.Errorf("wait script missing %q", want)
		}
	}

	// The migration container itself is unchanged: image entrypoint + `true`.
	mains := job.Spec.Template.Spec.Containers
	if len(mains) != 1 || mains[0].Name != "langfuse-migrate" {
		t.Fatalf("unexpected migration containers: %+v", mains)
	}
	if len(mains[0].Args) != 1 || mains[0].Args[0] != "true" {
		t.Errorf("migration container args = %v, want [true]", mains[0].Args)
	}
}
