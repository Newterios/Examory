# Files service

A student requests a signed MinIO form, uploads bytes directly to the private `submissions` bucket, then confirms the upload. The file service checks the student's identity at confirmation and download. Configure `FILES_DATABASE_URL`, `FILES_JWKS_URL`, `FILES_KAFKA_BROKERS`, `FILES_INTERNAL_TOKEN`, `MINIO_ENDPOINT`, `MINIO_PUBLIC_ENDPOINT`, `MINIO_ACCESS_KEY` and `MINIO_SECRET_KEY`. The public endpoint must be reachable by the client using the same hostname included in the signature. Explicit endpoint URLs select HTTP or HTTPS. Use TLS outside local development.

1. With the student's bearer token, POST `/api/files/v1/uploads` with `{"name":"essay.txt","mime":"text/plain","size":12}`. Use the UTF-8 byte count.
2. POST the returned fields, followed by the `file` part, as multipart form data to the returned URL. The signature covers one staging key, exact byte count, MIME type and a five-minute deadline.
3. POST `/api/files/v1/uploads/{file_id}/complete` with the same student's token. The receipt contains `file_id`, `name`, `size` and `sha256`. Include the ID when submitting an assignment. GET `/api/files/v1/files/{file_id}` downloads the completed copy.

Each grant permits up to 2 MiB of nonempty UTF-8. Supported MIME types are `text/plain`, `text/markdown`, `text/x-python` and `text/x-java-source`, with matching extensions. The service rejects control bytes (except tab and line endings), directory components and mismatched extensions. This version supports textual source files and documents. Treat accepted content as untrusted when displaying it. Set a bucket lifecycle rule on `uploads/` for expired staging objects on long-running installations.

On confirmation, files copies verified bytes to an immutable digest-keyed object. PostgreSQL records this digest and enqueues one `file.uploaded` event in `lms.submissions` in the same transaction. Retrying confirmation returns the original receipt. The assignments service uses authenticated internal routes to resolve an owner-bound completed file and conditionally create `{submission_id}/extracted.txt`. A repeat with identical bytes succeeds, different bytes return 409.

Run `go test ./...` in this module. To exercise signed multipart uploads and immutable reads, set `IT_PG_ADMIN_URL` to a disposable PostgreSQL instance and the MinIO variables above to a private test bucket with the project writer policy. Run `go test -tags integration -count=1 -run TestUploadsInPostgresAndMinIO ./internal/service`. The test creates randomly named objects in that bucket.
