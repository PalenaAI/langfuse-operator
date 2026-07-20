/*
Copyright 2026 bitkaio LLC.

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

package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/PalenaAI/langfuse-operator/api/v1alpha1"
)

// clickHouseQueryTimeout bounds a single DDL/query round-trip. Retention ALTERs
// are metadata-only operations, so they return quickly even on large tables.
const clickHouseQueryTimeout = 15 * time.Second

// defaultClickHouseDatabase matches Langfuse's CLICKHOUSE_DB default.
const defaultClickHouseDatabase = "default"

// clickHouseClient executes statements against ClickHouse's HTTP interface.
//
// The operator talks to ClickHouse directly (rather than through Langfuse) for
// retention DDL, schema inspection, and storage metrics. When the connection is
// TLS-protected by a private CA, the CA is loaded from the referenced Secret —
// the operator can read the Secret even though the certificate file itself is
// only mounted into the Langfuse pods.
type clickHouseClient struct {
	endpoint string
	user     string
	password string
	database string
	http     *http.Client
}

// newClickHouseClient resolves the ClickHouse endpoint, credentials, and TLS
// trust for an instance.
func newClickHouseClient(ctx context.Context, c client.Client, instance *v1alpha1.LangfuseInstance) (*clickHouseClient, error) {
	endpoint, err := resolveClickHouseURL(ctx, c, instance)
	if err != nil {
		return nil, fmt.Errorf("resolving ClickHouse URL: %w", err)
	}

	user, password, err := resolveClickHouseCredentials(ctx, c, instance)
	if err != nil {
		return nil, fmt.Errorf("resolving ClickHouse credentials: %w", err)
	}

	httpClient := &http.Client{Timeout: clickHouseQueryTimeout}
	if strings.HasPrefix(endpoint, "https://") {
		tlsConfig, err := instanceTLSConfig(ctx, c, instance)
		if err != nil {
			return nil, fmt.Errorf("building ClickHouse TLS config: %w", err)
		}
		if tlsConfig != nil {
			httpClient.Transport = &http.Transport{TLSClientConfig: tlsConfig}
		}
	}

	return &clickHouseClient{
		endpoint: endpoint,
		user:     user,
		password: password,
		database: defaultClickHouseDatabase,
		http:     httpClient,
	}, nil
}

// exec runs a statement and returns the raw response body. ClickHouse's HTTP
// interface reports errors as a non-2xx status with the message in the body.
func (q *clickHouseClient) exec(ctx context.Context, statement string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, clickHouseQueryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, q.endpoint, strings.NewReader(statement))
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	if q.user != "" {
		req.Header.Set("X-ClickHouse-User", q.user)
	}
	if q.password != "" {
		req.Header.Set("X-ClickHouse-Key", q.password)
	}
	if q.database != "" {
		req.Header.Set("X-ClickHouse-Database", q.database)
	}

	resp, err := q.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("clickhouse request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Cap the body: a malformed query can echo a very large error back.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("reading clickhouse response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("clickhouse returned HTTP %d: %s",
			resp.StatusCode, truncate(strings.TrimSpace(string(body)), 512))
	}
	return string(body), nil
}

// queryRows runs a query and splits the response into non-empty lines. Callers
// should append FORMAT TabSeparated (the HTTP default) and split fields on tab.
func (q *clickHouseClient) queryRows(ctx context.Context, statement string) ([]string, error) {
	body, err := q.exec(ctx, statement)
	if err != nil {
		return nil, err
	}
	var rows []string
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			rows = append(rows, line)
		}
	}
	return rows, nil
}

// resolveClickHouseCredentials returns the ClickHouse user and password,
// mirroring how internal/langfuse/config.go wires CLICKHOUSE_USER/PASSWORD.
func resolveClickHouseCredentials(ctx context.Context, c client.Client, instance *v1alpha1.LangfuseInstance) (string, string, error) {
	if instance.Spec.ClickHouse == nil {
		return "", "", fmt.Errorf("no clickhouse configured")
	}
	ch := instance.Spec.ClickHouse

	switch {
	case ch.Managed != nil:
		secretName := instance.Name + "-generated-secrets"
		userKey, passKey := "clickhouse-username", "clickhouse-password"
		if ch.Managed.Auth != nil && ch.Managed.Auth.SecretRef != nil {
			secretName = ch.Managed.Auth.SecretRef.Name
			if k := ch.Managed.Auth.SecretRef.Keys["username"]; k != "" {
				userKey = k
			}
			if k := ch.Managed.Auth.SecretRef.Keys["password"]; k != "" {
				passKey = k
			}
		}
		user, err := readSecretValue(ctx, c, instance.Namespace, secretName, userKey)
		if err != nil {
			return "", "", err
		}
		password, err := readSecretValue(ctx, c, instance.Namespace, secretName, passKey)
		if err != nil {
			return "", "", err
		}
		return user, password, nil

	case ch.External != nil:
		// Username and password are optional for external ClickHouse — an
		// unauthenticated dev server is valid, so a missing key is not an error.
		var user, password string
		if k := ch.External.SecretRef.Keys["username"]; k != "" {
			if v, err := readSecretValue(ctx, c, instance.Namespace, ch.External.SecretRef.Name, k); err == nil {
				user = v
			}
		}
		if k := ch.External.SecretRef.Keys["password"]; k != "" {
			if v, err := readSecretValue(ctx, c, instance.Namespace, ch.External.SecretRef.Name, k); err == nil {
				password = v
			}
		}
		return user, password, nil
	}

	return "", "", fmt.Errorf("no clickhouse mode set")
}

// instanceTLSConfig builds a TLS config trusting the instance's configured CA.
// Returns nil when no custom CA is configured, so the caller falls back to the
// system trust store (correct for a publicly-trusted certificate).
//
// Without this, the operator's own probes and queries fail x509 verification
// against a datastore secured by a private CA, even though the Langfuse pods —
// which mount the CA — connect fine.
func instanceTLSConfig(ctx context.Context, c client.Client, instance *v1alpha1.LangfuseInstance) (*tls.Config, error) {
	if instance.Spec.TLS == nil || instance.Spec.TLS.TrustedCASecretRef == nil {
		return nil, nil
	}

	ref := instance.Spec.TLS.TrustedCASecretRef
	key := ref.Key
	if key == "" {
		key = "ca.crt"
	}

	pem, err := readSecretValue(ctx, c, instance.Namespace, ref.Name, key)
	if err != nil {
		return nil, fmt.Errorf("reading trusted CA: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(pem)) {
		return nil, fmt.Errorf("trusted CA secret %s key %q contains no valid PEM certificate", ref.Name, key)
	}

	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, nil
}
