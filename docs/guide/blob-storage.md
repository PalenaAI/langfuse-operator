# Blob Storage

Langfuse uses blob storage for event upload and large trace payloads. The operator supports **S3**, **Azure Blob Storage**, and **Google Cloud Storage**.

::: warning Required
Blob storage is **mandatory** in Langfuse v3. The application will not start without a configured blob storage backend.
:::

## S3 / S3-Compatible

```yaml
spec:
  blobStorage:
    provider: s3
    s3:
      bucket: langfuse-traces
      region: us-east-1
      endpoint: ""                    # optional, for S3-compatible stores (MinIO, R2)
      forcePathStyle: false           # set true for MinIO / non-AWS
      credentials:
        secretRef:
          name: langfuse-s3
          keys:
            accessKeyId: access_key
            secretAccessKey: secret_key
```

On **EKS**, you can use IRSA or Pod Identity instead of static credentials by omitting the `credentials` block and annotating the ServiceAccount.

## Azure Blob Storage

```yaml
spec:
  blobStorage:
    provider: azure
    azure:
      storageAccountName: langfusedata
      containerName: traces
      # endpoint: https://langfusedata.blob.core.windows.net   # optional; derived from the account name by default
      credentials:
        secretRef:
          name: langfuse-azure
          keys:
            accountKey: account-key   # the storage account key (key name defaults to "accountKey")
```

::: warning Account key, not a connection string
Langfuse v3 authenticates to Azure with the **storage account key**, not an Azure
connection string. Store the account key (the `key1`/`key2` value from the storage
account's *Access keys* blade) in the referenced Secret. The operator maps the
container to Langfuse's upload bucket, the account name to the access key ID, and
the account key to the secret access key, and sets `LANGFUSE_USE_AZURE_BLOB=true`.
:::

For Azure Government or sovereign clouds, set `azure.endpoint` to the matching
blob service URL (e.g. `https://<account>.blob.core.usgovcloudapi.net`).

## Google Cloud Storage

```yaml
spec:
  blobStorage:
    provider: gcs
    gcs:
      bucketName: langfuse-traces
      projectId: my-gcp-project
      credentials:
        secretRef:
          name: langfuse-gcs
          keys:
            credentials: service-account.json   # inline service-account JSON (key name defaults to "credentials")
```

On **GKE**, you can use Workload Identity instead of static credentials — omit the
`credentials` block and Langfuse falls back to Application Default Credentials.
