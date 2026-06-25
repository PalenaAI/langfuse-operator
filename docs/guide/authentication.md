# Authentication

## Core Settings

Every `LangfuseInstance` requires a `NEXTAUTH_URL` and secrets for session encryption:

```yaml
spec:
  auth:
    nextAuthUrl: "https://langfuse.example.com"

    # Provide explicit references, or omit to auto-generate
    nextAuthSecret:
      secretRef:
        name: langfuse-auth
        key: nextauth-secret
    salt:
      secretRef:
        name: langfuse-auth
        key: salt
```

If you omit `nextAuthSecret` and `salt`, the operator generates random values and stores them in the `<instance>-generated-secrets` Secret.

## Email / Password

Enabled by default. To customize:

```yaml
spec:
  auth:
    emailPassword:
      enabled: true          # default: true
      disableSignup: false   # default: false, set true to block self-registration
```

To disable email/password entirely (e.g., OIDC-only):

```yaml
spec:
  auth:
    emailPassword:
      enabled: false
```

## OpenID Connect (OIDC)

The operator configures Langfuse's generic **custom OIDC provider**, mapping the
fields below to the upstream `AUTH_CUSTOM_*` environment variables.

```yaml
spec:
  auth:
    oidc:
      enabled: true
      issuer: "https://auth.example.com"
      clientId:
        name: oidc-secret
        key: client-id
      clientSecret:
        name: oidc-secret
        key: client-secret
      name: "Acme SSO"              # optional, login button label (default "SSO")
      scope:                        # optional, default ["openid", "email", "profile"]
        - openid
        - email
        - profile
      ssoEnforcedDomains:           # optional, domains forced to use SSO
        - example.com
```

| Field | Upstream variable | Notes |
| --- | --- | --- |
| `issuer` | `AUTH_CUSTOM_ISSUER` | OIDC issuer URL |
| `clientId` | `AUTH_CUSTOM_CLIENT_ID` | from Secret |
| `clientSecret` | `AUTH_CUSTOM_CLIENT_SECRET` | from Secret |
| `name` | `AUTH_CUSTOM_NAME` | login button label, defaults to `SSO` |
| `scope` | `AUTH_CUSTOM_SCOPE` | space-joined, defaults to `openid email profile` |
| `ssoEnforcedDomains` | `AUTH_DOMAINS_WITH_SSO_ENFORCEMENT` | comma-joined; these domains may **only** sign in via SSO (password login disabled). This is a global setting, not a per-provider allow-list — upstream Langfuse has no generic custom-OIDC allowed-domains variable. |

::: warning Callback URL
Whitelist the redirect URL `<NEXTAUTH_URL>/api/auth/callback/custom` in your
identity provider (e.g. `https://langfuse.example.com/api/auth/callback/custom`).
:::

Create the OIDC Secret:

```bash
kubectl create secret generic oidc-secret -n langfuse \
  --from-literal=client-id="your-client-id" \
  --from-literal=client-secret="your-client-secret"
```

## Initial Admin User

Bootstrap an admin user on first deployment:

```yaml
spec:
  auth:
    initUser:
      enabled: true
      email: "admin@example.com"
      password:
        name: langfuse-init
        key: password
      orgName: "Default"
      projectName: "Default"
```

The init user is only created if no users exist in the database. It's safe to leave this enabled across reconciles.
