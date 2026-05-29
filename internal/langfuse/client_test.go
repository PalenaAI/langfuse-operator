/*
Copyright 2026 bitkaio LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package langfuse

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

// capture records the method, path, and Authorization header of the last request.
type capture struct {
	method string
	path   string
	auth   string
}

func newCapturingServer(t *testing.T, status int, body string, cap *capture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.method = r.Method
		cap.path = r.URL.RequestURI()
		cap.auth = r.Header.Get("Authorization")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestAdminClient_CreateOrganization_BearerAuth(t *testing.T) {
	var cap capture
	srv := newCapturingServer(t, http.StatusOK, `{"id":"org-1","name":"acme"}`, &cap)
	defer srv.Close()

	c := NewAdminClient(srv.URL, "admin-key-123")
	org, err := c.CreateOrganization(context.Background(), "acme")
	if err != nil {
		t.Fatalf("CreateOrganization error: %v", err)
	}
	if org.ID != "org-1" {
		t.Errorf("org ID = %q, want org-1", org.ID)
	}
	if cap.method != http.MethodPost || cap.path != "/api/admin/organizations" {
		t.Errorf("got %s %s, want POST /api/admin/organizations", cap.method, cap.path)
	}
	if cap.auth != "Bearer admin-key-123" {
		t.Errorf("auth = %q, want Bearer admin-key-123", cap.auth)
	}
}

func TestAdminClient_CreateOrganizationAPIKey_Path(t *testing.T) {
	var cap capture
	srv := newCapturingServer(t, http.StatusCreated, `{"id":"k1","publicKey":"pk","secretKey":"sk"}`, &cap)
	defer srv.Close()

	c := NewAdminClient(srv.URL, "admin-key-123")
	pair, err := c.CreateOrganizationAPIKey(context.Background(), "org-1", "note")
	if err != nil {
		t.Fatalf("CreateOrganizationAPIKey error: %v", err)
	}
	if pair.PublicKey != "pk" || pair.SecretKey != "sk" {
		t.Errorf("got pk=%q sk=%q", pair.PublicKey, pair.SecretKey)
	}
	if cap.method != http.MethodPost || cap.path != "/api/admin/organizations/org-1/apiKeys" {
		t.Errorf("got %s %s, want POST /api/admin/organizations/org-1/apiKeys", cap.method, cap.path)
	}
	if cap.auth != "Bearer admin-key-123" {
		t.Errorf("auth = %q, want Bearer", cap.auth)
	}
}

func TestProjectClient_CreateProject_PublicAPIBasicAuth(t *testing.T) {
	var cap capture
	srv := newCapturingServer(t, http.StatusCreated, `{"id":"proj-1","name":"web"}`, &cap)
	defer srv.Close()

	c := NewProjectClient(srv.URL, "pk-org", "sk-org")
	proj, err := c.CreateProject(context.Background(), "web")
	if err != nil {
		t.Fatalf("CreateProject error: %v", err)
	}
	if proj.ID != "proj-1" {
		t.Errorf("project ID = %q, want proj-1", proj.ID)
	}
	if cap.method != http.MethodPost || cap.path != "/api/public/projects" {
		t.Errorf("got %s %s, want POST /api/public/projects", cap.method, cap.path)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk-org:sk-org"))
	if cap.auth != wantAuth {
		t.Errorf("auth = %q, want %q", cap.auth, wantAuth)
	}
}

func TestProjectClient_ListProjects_ParsesProjectsKey(t *testing.T) {
	var cap capture
	srv := newCapturingServer(t, http.StatusOK, `{"projects":[{"id":"p1","name":"a"},{"id":"p2","name":"b"}]}`, &cap)
	defer srv.Close()

	c := NewProjectClient(srv.URL, "pk", "sk")
	projects, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects error: %v", err)
	}
	if len(projects) != 2 || projects[0].ID != "p1" || projects[1].Name != "b" {
		t.Fatalf("unexpected projects: %+v", projects)
	}
	if cap.path != "/api/public/projects" {
		t.Errorf("path = %q, want /api/public/projects", cap.path)
	}
}

func TestProjectClient_CreateAPIKey_PublicPath(t *testing.T) {
	var cap capture
	srv := newCapturingServer(t, http.StatusCreated, `{"id":"ak1","publicKey":"pk-lf","secretKey":"sk-lf"}`, &cap)
	defer srv.Close()

	c := NewProjectClient(srv.URL, "pk", "sk")
	pair, err := c.CreateAPIKey(context.Background(), "proj-1", "note")
	if err != nil {
		t.Fatalf("CreateAPIKey error: %v", err)
	}
	if pair.SecretKey != "sk-lf" {
		t.Errorf("secretKey = %q, want sk-lf", pair.SecretKey)
	}
	if cap.method != http.MethodPost || cap.path != "/api/public/projects/proj-1/apiKeys" {
		t.Errorf("got %s %s, want POST /api/public/projects/proj-1/apiKeys", cap.method, cap.path)
	}
}

func TestProjectClient_DeleteAPIKey_ProjectScopedPath(t *testing.T) {
	var cap capture
	srv := newCapturingServer(t, http.StatusOK, `{}`, &cap)
	defer srv.Close()

	c := NewProjectClient(srv.URL, "pk", "sk")
	if err := c.DeleteAPIKey(context.Background(), "proj-1", "ak1"); err != nil {
		t.Fatalf("DeleteAPIKey error: %v", err)
	}
	if cap.method != http.MethodDelete || cap.path != "/api/public/projects/proj-1/apiKeys/ak1" {
		t.Errorf("got %s %s, want DELETE /api/public/projects/proj-1/apiKeys/ak1", cap.method, cap.path)
	}
}
