# Optional S3-compatible storage

The base deployment stores attachments in the `app-uploads` volume. To exercise the optional S3-compatible adapter, start MinIO and explicitly configure the application:

```sh
docker compose --profile object-storage up -d minio
S3_ENDPOINT=http://minio:9000 S3_BUCKET=crm-local docker compose up -d app
```

The Compose credentials are local-development defaults. Replace them before exposing MinIO beyond a trusted development machine.
