# Blob Storage

Langfuse uses blob storage for large trace payloads. The operator supports **S3**, **Azure Blob Storage**, and **Google Cloud Storage**.

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
      credentials:
        secretRef:
          name: langfuse-azure
```

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
```

On **GKE**, you can use Workload Identity instead of static credentials.
