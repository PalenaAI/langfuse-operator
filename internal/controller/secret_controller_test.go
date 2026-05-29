/*
Copyright 2026 bitkaio LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

func newSecretController(t *testing.T, objs ...interface{}) *SecretController {
	t.Helper()
	clientObjs := make([]client.Object, 0, len(objs))
	for _, o := range objs {
		clientObjs = append(clientObjs, o.(client.Object))
	}
	c := newFakeClient(t, clientObjs...)
	return &SecretController{Client: c, Scheme: c.Scheme()}
}

func TestEnsureGeneratedSecret_CreatesWithAllKeys(t *testing.T) {
	instance := &v1alpha1.LangfuseInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "lf", Namespace: "ns"},
	}
	r := newSecretController(t, instance)

	if err := r.ensureGeneratedSecret(context.Background(), instance, "lf-generated-secrets"); err != nil {
		t.Fatalf("ensureGeneratedSecret error: %v", err)
	}

	got := &corev1.Secret{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "lf-generated-secrets", Namespace: "ns"}, got); err != nil {
		t.Fatalf("get secret: %v", err)
	}
	for _, key := range []string{"nextauth-secret", "salt", "clickhouse-username", "clickhouse-password", "redis-password", "admin-api-key"} {
		if len(got.Data[key]) == 0 {
			t.Errorf("generated secret missing key %q", key)
		}
	}
}

// TestEnsureGeneratedSecret_BackfillsMissingKeys simulates an upgrade: a
// pre-0.7.0 secret without admin-api-key must get the key added rather than
// being left untouched (otherwise Langfuse pods fail with
// CreateContainerConfigError on the missing env reference).
func TestEnsureGeneratedSecret_BackfillsMissingKeys(t *testing.T) {
	instance := &v1alpha1.LangfuseInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "lf", Namespace: "ns"},
	}
	preexisting := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "lf-generated-secrets", Namespace: "ns"},
		Data: map[string][]byte{
			"nextauth-secret":     []byte("existing-nas"),
			"salt":                []byte("existing-salt"),
			"clickhouse-username": []byte("default"),
			"clickhouse-password": []byte("existing-chpw"),
			"redis-password":      []byte("existing-redispw"),
			// admin-api-key intentionally absent (pre-0.7.0 secret)
		},
	}
	r := newSecretController(t, instance, preexisting)

	if err := r.ensureGeneratedSecret(context.Background(), instance, "lf-generated-secrets"); err != nil {
		t.Fatalf("ensureGeneratedSecret error: %v", err)
	}

	got := &corev1.Secret{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "lf-generated-secrets", Namespace: "ns"}, got); err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if len(got.Data["admin-api-key"]) == 0 {
		t.Error("admin-api-key was not backfilled into the existing secret")
	}
	// Existing values must be preserved, not regenerated.
	if string(got.Data["nextauth-secret"]) != "existing-nas" {
		t.Errorf("nextauth-secret was overwritten: %q", got.Data["nextauth-secret"])
	}
	if string(got.Data["clickhouse-password"]) != "existing-chpw" {
		t.Errorf("clickhouse-password was overwritten: %q", got.Data["clickhouse-password"])
	}
}
