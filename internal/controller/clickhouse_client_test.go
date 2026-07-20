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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

func chInstance() *v1alpha1.LangfuseInstance {
	return &v1alpha1.LangfuseInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
		Spec: v1alpha1.LangfuseInstanceSpec{
			Image: v1alpha1.ImageSpec{Tag: "3"},
			Auth:  v1alpha1.AuthSpec{NextAuthUrl: "https://langfuse.example.com"},
			ClickHouse: &v1alpha1.ClickHouseSpec{
				External: &v1alpha1.ExternalClickHouseSpec{
					SecretRef: v1alpha1.SecretKeysRef{
						Name: "ch-creds",
						Keys: map[string]string{"url": "url", "username": "username", "password": "password"},
					},
				},
			},
		},
	}
}

func chSecret(url string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ch-creds", Namespace: "ns"},
		Data: map[string][]byte{
			"url":      []byte(url),
			"username": []byte("default"),
			"password": []byte("s3cret"),
		},
	}
}

func TestClickHouseClient_ExecSendsAuthAndQuery(t *testing.T) {
	var gotBody, gotUser, gotKey, gotDB string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		gotUser = r.Header.Get("X-ClickHouse-User")
		gotKey = r.Header.Get("X-ClickHouse-Key")
		gotDB = r.Header.Get("X-ClickHouse-Database")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	instance := chInstance()
	c := newFakeClient(t, chSecret(srv.URL))

	ch, err := newClickHouseClient(context.Background(), c, instance)
	if err != nil {
		t.Fatalf("newClickHouseClient() error: %v", err)
	}
	if _, err := ch.exec(context.Background(), "ALTER TABLE traces MODIFY TTL timestamp + INTERVAL 30 DAY"); err != nil {
		t.Fatalf("exec() error: %v", err)
	}

	if !strings.Contains(gotBody, "MODIFY TTL") {
		t.Errorf("body = %q, want the statement", gotBody)
	}
	if gotUser != "default" || gotKey != "s3cret" {
		t.Errorf("auth headers = %q/%q, want default/s3cret", gotUser, gotKey)
	}
	if gotDB != defaultClickHouseDatabase {
		t.Errorf("database header = %q, want %q", gotDB, defaultClickHouseDatabase)
	}
}

// A failed DDL must surface as an error, not be swallowed into a success that
// makes status claim retention was applied.
func TestClickHouseClient_ExecPropagatesServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Code: 60. DB::Exception: Table default.traces does not exist"))
	}))
	defer srv.Close()

	ch, err := newClickHouseClient(context.Background(), newFakeClient(t, chSecret(srv.URL)), chInstance())
	if err != nil {
		t.Fatalf("newClickHouseClient() error: %v", err)
	}

	_, err = ch.exec(context.Background(), "ALTER TABLE traces MODIFY TTL x")
	if err == nil {
		t.Fatal("expected an error for a 400 response")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q should include the ClickHouse message", err)
	}
}

func TestClickHouseClient_QueryRowsSplitsAndTrims(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("traces\nobservations\n\nscores\n"))
	}))
	defer srv.Close()

	ch, err := newClickHouseClient(context.Background(), newFakeClient(t, chSecret(srv.URL)), chInstance())
	if err != nil {
		t.Fatalf("newClickHouseClient() error: %v", err)
	}

	rows, err := ch.queryRows(context.Background(), "SELECT name FROM system.tables")
	if err != nil {
		t.Fatalf("queryRows() error: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %v, want 3 non-empty entries", rows)
	}
}

func TestInstanceTLSConfig(t *testing.T) {
	t.Run("nil when no trusted CA configured", func(t *testing.T) {
		cfg, err := instanceTLSConfig(context.Background(), newFakeClient(t), chInstance())
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if cfg != nil {
			t.Error("want nil config so the system trust store is used")
		}
	})

	t.Run("rejects a secret with no valid PEM", func(t *testing.T) {
		instance := chInstance()
		instance.Spec.TLS = &v1alpha1.TLSSpec{
			TrustedCASecretRef: &v1alpha1.CACertSecretRef{Name: "ca-bundle"},
		}
		bad := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "ca-bundle", Namespace: "ns"},
			Data:       map[string][]byte{"ca.crt": []byte("not a certificate")},
		}
		if _, err := instanceTLSConfig(context.Background(), newFakeClient(t, bad), instance); err == nil {
			t.Fatal("expected an error for a secret containing no valid PEM")
		}
	})
}

func TestResolveClickHouseCredentials_ExternalWithoutAuth(t *testing.T) {
	// An unauthenticated external ClickHouse is valid — missing username and
	// password keys must not be treated as an error.
	instance := chInstance()
	instance.Spec.ClickHouse.External.SecretRef.Keys = map[string]string{"url": "url"}

	user, password, err := resolveClickHouseCredentials(
		context.Background(), newFakeClient(t, chSecret("http://ch:8123")), instance)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if user != "" || password != "" {
		t.Errorf("got %q/%q, want empty credentials", user, password)
	}
}

func TestHumanizeBytes(t *testing.T) {
	for _, tc := range []struct {
		in   uint64
		want string
	}{
		{512, "512B"},
		{1024, "1.0Ki"},
		{1536, "1.5Ki"},
		{1024 * 1024, "1.0Mi"},
		{3 * 1024 * 1024 * 1024, "3.0Gi"},
	} {
		if got := humanizeBytes(tc.in); got != tc.want {
			t.Errorf("humanizeBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ─── Deprecation of managed datastore modes ─────────────────────────────────

func TestSetDeprecationCondition(t *testing.T) {
	ctx := context.Background()

	t.Run("no condition when only external modes are used", func(t *testing.T) {
		instance := chInstance()
		setDeprecationCondition(ctx, instance)
		if c := findCondition(instance); c != nil {
			t.Errorf("unexpected Deprecated condition: %+v", c)
		}
	})

	t.Run("flags managed clickhouse and redis", func(t *testing.T) {
		instance := chInstance()
		instance.Spec.ClickHouse = &v1alpha1.ClickHouseSpec{Managed: &v1alpha1.ManagedClickHouseSpec{}}
		instance.Spec.Redis = &v1alpha1.RedisSpec{Managed: &v1alpha1.ManagedRedisSpec{}}

		setDeprecationCondition(ctx, instance)

		c := findCondition(instance)
		if c == nil {
			t.Fatal("expected a Deprecated condition")
		}
		for _, want := range []string{"spec.clickhouse.managed", "spec.redis.managed", "0.11.0"} {
			if !strings.Contains(c.Message, want) {
				t.Errorf("message %q missing %q", c.Message, want)
			}
		}
	})

	t.Run("clears the condition once managed modes are removed", func(t *testing.T) {
		instance := chInstance()
		instance.Spec.Redis = &v1alpha1.RedisSpec{Managed: &v1alpha1.ManagedRedisSpec{}}
		setDeprecationCondition(ctx, instance)
		if findCondition(instance) == nil {
			t.Fatal("expected a Deprecated condition")
		}

		instance.Spec.Redis = &v1alpha1.RedisSpec{
			External: &v1alpha1.ExternalRedisSpec{SecretRef: v1alpha1.SecretKeysRef{Name: "r"}},
		}
		setDeprecationCondition(ctx, instance)
		if c := findCondition(instance); c != nil {
			t.Errorf("condition should be cleared, got %+v", c)
		}
	})
}

func findCondition(instance *v1alpha1.LangfuseInstance) *metav1.Condition {
	for i := range instance.Status.Conditions {
		if instance.Status.Conditions[i].Type == conditionTypeDeprecated {
			return &instance.Status.Conditions[i]
		}
	}
	return nil
}
