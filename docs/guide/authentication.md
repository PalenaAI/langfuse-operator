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
      allowedDomains:
        - example.com
```

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
