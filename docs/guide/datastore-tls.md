# Datastore TLS

Encrypt the connections from Langfuse to its backing datastores — PostgreSQL,
ClickHouse, and Redis — trusting a caller-supplied CA, optionally with a client
certificate for mutual TLS.

This is east-west (in-cluster) encryption between the Langfuse pods and their
data plane. It is independent of the ingress/Route TLS that terminates user
traffic (see [Networking](./networking)).

::: tip Web **and** Worker
Every TLS env var and volume is applied to **both** the Web and Worker pods (and
the migration Job). The Worker does most of the Redis/ClickHouse work, so TLS
that only reached the Web component would break ingestion under encryption.
:::

## How it works

There are two layers, designed to compose:

1. **A trusted CA mount** (`spec.tls.trustedCASecretRef`) — the simplest knob,
   and enough on its own for ClickHouse. The referenced CA is mounted into the
   pods and exported as `NODE_EXTRA_CA_CERTS`, which makes the Node.js runtime
   trust it for **all** outbound TLS. It also becomes the default CA for the
   Redis and PostgreSQL connections when those don't specify their own.
2. **Per-connection TLS blocks** (`redis.external.tls`, `clickhouse.external.tls`,
   `database.external.tls`) — protocol-specific wiring on top: enabling TLS,
   pointing each client at its CA, SNI overrides, mTLS client certs, and the
   PostgreSQL SSL mode.

All references are secret-ref based (`name` + `key`) and accept cert-manager's
standard `ca.crt` / `tls.crt` / `tls.key` keys by default.

## Trusted CA bundle

```yaml
spec:
  tls:
    trustedCASecretRef:
      name: internal-ca-bundle
      key: ca.crt            # default
```

This single mechanism unblocks **ClickHouse HTTPS** (which has no CA-path env of
its own) and provides the default CA for PostgreSQL and Redis verification.

## Redis

```yaml
spec:
  redis:
    external:
      secretRef:
        name: langfuse-redis-conn
        keys: { host: host, port: port, password: password }
      tls:
        enabled: true
        # caSecretRef omitted → uses spec.tls.trustedCASecretRef
        clientCertSecretRef:        # optional, for mutual TLS
          name: langfuse-redis-tls  # cert-manager Secret: tls.crt + tls.key
        serverName: redis.example.com   # optional SNI override
```

Translates to (on web + worker): `REDIS_TLS_ENABLED=true`, plus
`REDIS_TLS_CA_PATH`, `REDIS_TLS_CERT_PATH`, `REDIS_TLS_KEY_PATH`, and
`REDIS_TLS_SERVERNAME` when the corresponding fields are set.

::: warning Redis ignores `NODE_EXTRA_CA_CERTS`
Langfuse's Redis client (ioredis) reads the CA from `REDIS_TLS_CA_PATH`
directly — it does **not** consult the Node trust store. The operator therefore
always points `REDIS_TLS_CA_PATH` at the per-connection CA, or at the trusted CA
bundle as a fallback. With neither set, verification falls back to the system
trust store (fine for Redis served by a publicly-trusted CA).
:::

A legacy alternative remains supported: a boolean `tls` key in the connection
Secret (`keys: { tls: tls_enabled }`) maps to `REDIS_TLS_ENABLED`. Prefer the
typed `tls` block, which also handles the CA, client cert, and SNI.

## ClickHouse

```yaml
spec:
  clickhouse:
    external:
      secretRef:
        name: langfuse-clickhouse-conn
        keys:
          url: url                # https://clickhouse.example.com:8443
          migrationUrl: migration_url   # clickhouse://clickhouse.example.com:9440
          username: username
          password: password
      tls:
        enabled: true
```

When enabled, the operator sets `CLICKHOUSE_MIGRATION_SSL=true`. CA trust for the
runtime HTTPS client comes from `NODE_EXTRA_CA_CERTS` (the trusted CA bundle) —
ClickHouse has no dedicated CA-path variable.

::: warning Use the TLS scheme/port in the Secret
The operator does not rewrite the URLs in your Secret. Make sure `CLICKHOUSE_URL`
uses `https://…:8443` and `CLICKHOUSE_MIGRATION_URL` uses the native secure
endpoint `clickhouse://…:9440`.
:::

## PostgreSQL

```yaml
spec:
  database:
    external:
      secretRef:
        name: langfuse-postgres-conn
        keys: { url: database_url }   # must NOT contain a query string
      tls:
        sslMode: verify-full     # disable | require | verify-ca | verify-full
        caSecretRef:             # optional; defaults to spec.tls.trustedCASecretRef
          name: langfuse-postgres-tls
```

Langfuse connects via Prisma. Because the connection string lives in a Secret,
the operator does not edit it in place; instead it sources the base URL into
`DATABASE_URL_BASE` and composes the effective `DATABASE_URL` as
`$(DATABASE_URL_BASE)?<tls params>` using Kubernetes env-var interpolation. The
same applies to `DIRECT_URL` when a `directUrl` key is present.

::: warning No query string in the base URL
Because the operator appends `?…`, the URL stored in the Secret must not already
contain its own query string.
:::

`sslMode` maps to Prisma's connection parameters (Prisma does **not** use libpq's
`sslrootcert` or `PG*` env vars):

| `sslMode`     | Prisma parameters                                  |
|---------------|----------------------------------------------------|
| `disable`     | `sslmode=disable`                                  |
| `require`     | `sslmode=require&sslaccept=accept_invalid_certs`   |
| `verify-ca`   | `sslmode=require&sslaccept=strict`                 |
| `verify-full` | `sslmode=require&sslaccept=strict`                 |

The CA is referenced via Prisma's `sslcert` parameter. Prisma has no CA-only
mode, so `verify-ca` and `verify-full` both enable strict verification (CA chain
+ hostname).

## General escape hatch

For anything the typed blocks don't cover, both `spec.web` and `spec.worker`
accept `extraEnv`, `extraVolumes`, and `extraVolumeMounts`. Use these to mount
additional certificates or set extra `REDIS_TLS_*` / Prisma parameters by hand —
just remember to set them on **both** components.

```yaml
spec:
  worker:
    extraVolumes:
      - name: extra-ca
        secret: { secretName: another-ca }
    extraVolumeMounts:
      - name: extra-ca
        mountPath: /etc/extra-ca
        readOnly: true
    extraEnv:
      - name: REDIS_TLS_REJECT_UNAUTHORIZED
        value: "true"
```
