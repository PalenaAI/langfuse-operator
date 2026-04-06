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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Organization represents a Langfuse organization.
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OrgMember represents a member of a Langfuse organization.
type OrgMember struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Project represents a Langfuse project.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// APIKey represents a Langfuse API key (public portion only).
type APIKey struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
}

// APIKeyPair represents a newly created Langfuse API key pair
// including the secret key, which is only available at creation time.
type APIKeyPair struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	SecretKey string `json:"secretKey"`
}

// BackgroundMigrations represents the status of Langfuse background migrations.
type BackgroundMigrations struct {
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
}

// Client is the interface for the Langfuse Admin API.
type Client interface {
	Health(ctx context.Context) error
	ListOrganizations(ctx context.Context) ([]Organization, error)
	GetOrganization(ctx context.Context, id string) (*Organization, error)
	CreateOrganization(ctx context.Context, name string) (*Organization, error)
	UpdateOrganization(ctx context.Context, id string, name string) (*Organization, error)
	DeleteOrganization(ctx context.Context, id string) error
	ListOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error)
	AddOrgMember(ctx context.Context, orgID string, email string, role string) (*OrgMember, error)
	UpdateOrgMemberRole(ctx context.Context, orgID string, memberID string, role string) error
	RemoveOrgMember(ctx context.Context, orgID string, memberID string) error
	ListProjects(ctx context.Context, orgID string) ([]Project, error)
	GetProject(ctx context.Context, projectID string) (*Project, error)
	CreateProject(ctx context.Context, orgID string, name string) (*Project, error)
	DeleteProject(ctx context.Context, projectID string) error
	ListAPIKeys(ctx context.Context, projectID string) ([]APIKey, error)
	CreateAPIKey(ctx context.Context, projectID string, note string) (*APIKeyPair, error)
	DeleteAPIKey(ctx context.Context, apiKeyID string) error
	GetBackgroundMigrations(ctx context.Context) (*BackgroundMigrations, error)
}

// httpClient implements the Client interface using HTTP requests to the Langfuse Admin API.
type httpClient struct {
	baseURL    string
	authHeader string
	http       *http.Client
}

// NewClient creates a new Langfuse Admin API client.
// The baseURL should be the root URL of the Langfuse instance (e.g. http://instance-web.ns.svc:3000).
// Authentication uses HTTP Basic auth with the provided public and secret keys.
func NewClient(baseURL, publicKey, secretKey string) Client {
	baseURL = strings.TrimRight(baseURL, "/")
	credentials := base64.StdEncoding.EncodeToString([]byte(publicKey + ":" + secretKey))
	return &httpClient{
		baseURL:    baseURL,
		authHeader: "Basic " + credentials,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Health checks whether the Langfuse instance is healthy.
func (c *httpClient) Health(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/public/health", nil)
	if err != nil {
		return fmt.Errorf("checking health: %w", err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("checking health: %w", err)
	}
	return nil
}

// ListOrganizations returns all organizations.
func (c *httpClient) ListOrganizations(ctx context.Context) ([]Organization, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/admin/organizations", nil)
	if err != nil {
		return nil, fmt.Errorf("listing organizations: %w", err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("listing organizations: %w", err)
	}

	var result struct {
		Data []Organization `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("listing organizations: decoding response: %w", err)
	}
	return result.Data, nil
}

// GetOrganization returns a single organization by ID.
func (c *httpClient) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/admin/organizations/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("getting organization %q: %w", id, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("getting organization %q: %w", id, err)
	}

	var org Organization
	if err := json.NewDecoder(resp.Body).Decode(&org); err != nil {
		return nil, fmt.Errorf("getting organization %q: decoding response: %w", id, err)
	}
	return &org, nil
}

// CreateOrganization creates a new organization with the given name.
func (c *httpClient) CreateOrganization(ctx context.Context, name string) (*Organization, error) {
	body := map[string]string{"name": name}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/admin/organizations", body)
	if err != nil {
		return nil, fmt.Errorf("creating organization: %w", err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("creating organization: %w", err)
	}

	var org Organization
	if err := json.NewDecoder(resp.Body).Decode(&org); err != nil {
		return nil, fmt.Errorf("creating organization: decoding response: %w", err)
	}
	return &org, nil
}

// UpdateOrganization updates the name of an existing organization.
func (c *httpClient) UpdateOrganization(ctx context.Context, id string, name string) (*Organization, error) {
	body := map[string]string{"name": name}
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/admin/organizations/"+id, body)
	if err != nil {
		return nil, fmt.Errorf("updating organization %q: %w", id, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("updating organization %q: %w", id, err)
	}

	var org Organization
	if err := json.NewDecoder(resp.Body).Decode(&org); err != nil {
		return nil, fmt.Errorf("updating organization %q: decoding response: %w", id, err)
	}
	return &org, nil
}

// DeleteOrganization deletes an organization by ID.
func (c *httpClient) DeleteOrganization(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/admin/organizations/"+id, nil)
	if err != nil {
		return fmt.Errorf("deleting organization %q: %w", id, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("deleting organization %q: %w", id, err)
	}
	return nil
}

// ListOrgMembers returns all members of an organization.
func (c *httpClient) ListOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/admin/organizations/"+orgID+"/members", nil)
	if err != nil {
		return nil, fmt.Errorf("listing members for organization %q: %w", orgID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("listing members for organization %q: %w", orgID, err)
	}

	var result struct {
		Data []OrgMember `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("listing members for organization %q: decoding response: %w", orgID, err)
	}
	return result.Data, nil
}

// AddOrgMember adds a member to an organization with the specified role.
func (c *httpClient) AddOrgMember(ctx context.Context, orgID string, email string, role string) (*OrgMember, error) {
	body := map[string]string{"email": email, "role": role}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/admin/organizations/"+orgID+"/members", body)
	if err != nil {
		return nil, fmt.Errorf("adding member to organization %q: %w", orgID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("adding member to organization %q: %w", orgID, err)
	}

	var member OrgMember
	if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
		return nil, fmt.Errorf("adding member to organization %q: decoding response: %w", orgID, err)
	}
	return &member, nil
}

// UpdateOrgMemberRole updates the role of an organization member.
func (c *httpClient) UpdateOrgMemberRole(ctx context.Context, orgID string, memberID string, role string) error {
	body := map[string]string{"role": role}
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/admin/organizations/"+orgID+"/members/"+memberID, body)
	if err != nil {
		return fmt.Errorf("updating role for member %q in organization %q: %w", memberID, orgID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("updating role for member %q in organization %q: %w", memberID, orgID, err)
	}
	return nil
}

// RemoveOrgMember removes a member from an organization.
func (c *httpClient) RemoveOrgMember(ctx context.Context, orgID string, memberID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/admin/organizations/"+orgID+"/members/"+memberID, nil)
	if err != nil {
		return fmt.Errorf("removing member %q from organization %q: %w", memberID, orgID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("removing member %q from organization %q: %w", memberID, orgID, err)
	}
	return nil
}

// ListProjects returns all projects for an organization.
func (c *httpClient) ListProjects(ctx context.Context, orgID string) ([]Project, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/admin/projects?orgId="+orgID, nil)
	if err != nil {
		return nil, fmt.Errorf("listing projects for organization %q: %w", orgID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("listing projects for organization %q: %w", orgID, err)
	}

	var result struct {
		Data []Project `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("listing projects for organization %q: decoding response: %w", orgID, err)
	}
	return result.Data, nil
}

// GetProject returns a single project by ID.
func (c *httpClient) GetProject(ctx context.Context, projectID string) (*Project, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/admin/projects/"+projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("getting project %q: %w", projectID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("getting project %q: %w", projectID, err)
	}

	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("getting project %q: decoding response: %w", projectID, err)
	}
	return &project, nil
}

// CreateProject creates a new project in the specified organization.
func (c *httpClient) CreateProject(ctx context.Context, orgID string, name string) (*Project, error) {
	body := map[string]string{"orgId": orgID, "name": name}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/admin/projects", body)
	if err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("creating project: decoding response: %w", err)
	}
	return &project, nil
}

// DeleteProject deletes a project by ID.
func (c *httpClient) DeleteProject(ctx context.Context, projectID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/admin/projects/"+projectID, nil)
	if err != nil {
		return fmt.Errorf("deleting project %q: %w", projectID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("deleting project %q: %w", projectID, err)
	}
	return nil
}

// ListAPIKeys returns all API keys for a project.
func (c *httpClient) ListAPIKeys(ctx context.Context, projectID string) ([]APIKey, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/admin/projects/"+projectID+"/api-keys", nil)
	if err != nil {
		return nil, fmt.Errorf("listing API keys for project %q: %w", projectID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("listing API keys for project %q: %w", projectID, err)
	}

	var result struct {
		Data []APIKey `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("listing API keys for project %q: decoding response: %w", projectID, err)
	}
	return result.Data, nil
}

// CreateAPIKey creates a new API key pair for a project.
// The returned APIKeyPair includes the secret key, which is only available at creation time.
func (c *httpClient) CreateAPIKey(ctx context.Context, projectID string, note string) (*APIKeyPair, error) {
	body := map[string]string{"note": note}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/admin/projects/"+projectID+"/api-keys", body)
	if err != nil {
		return nil, fmt.Errorf("creating API key for project %q: %w", projectID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("creating API key for project %q: %w", projectID, err)
	}

	var keyPair APIKeyPair
	if err := json.NewDecoder(resp.Body).Decode(&keyPair); err != nil {
		return nil, fmt.Errorf("creating API key for project %q: decoding response: %w", projectID, err)
	}
	return &keyPair, nil
}

// DeleteAPIKey deletes an API key by ID.
func (c *httpClient) DeleteAPIKey(ctx context.Context, apiKeyID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/admin/api-keys/"+apiKeyID, nil)
	if err != nil {
		return fmt.Errorf("deleting API key %q: %w", apiKeyID, err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("deleting API key %q: %w", apiKeyID, err)
	}
	return nil
}

// GetBackgroundMigrations returns the status of background migrations.
func (c *httpClient) GetBackgroundMigrations(ctx context.Context) (*BackgroundMigrations, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/public/background-migrations", nil)
	if err != nil {
		return nil, fmt.Errorf("getting background migrations: %w", err)
	}
	defer closeBody(resp)

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("getting background migrations: %w", err)
	}

	var migrations BackgroundMigrations
	if err := json.NewDecoder(resp.Body).Decode(&migrations); err != nil {
		return nil, fmt.Errorf("getting background migrations: decoding response: %w", err)
	}
	return &migrations, nil
}

// doRequest builds and executes an HTTP request with authentication and optional JSON body.
func (c *httpClient) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	return resp, nil
}

// apiError represents an error response from the Langfuse API.
type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("API returned status %d: %s", e.StatusCode, e.Body)
}

// closeBody drains and closes an HTTP response body. The error is intentionally
// discarded because there is no meaningful recovery action for a close failure.
func closeBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

// checkResponse returns an error if the HTTP response status code is not in the 2xx range.
func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return &apiError{
		StatusCode: resp.StatusCode,
		Body:       strings.TrimSpace(string(body)),
	}
}
