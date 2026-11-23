# Text integrity worker

The worker consumes validated `submission.created` events from `lms.submissions`. It retrieves the exact UTF-8 object from a configured MinIO endpoint and compares normalised five-token shingles in 200-token windows at stride 100. PostgreSQL stores a durable inbox, 32 sets of four-row MinHash bucket keys per window, source checksums and a transactional outbox. Duplicate input and publisher retries preserve risk identity. Out-of-order arrivals are checked in both directions. Each verified pair produces one risk for its later submission, with submission ID breaking timestamp ties. Comparisons exclude an author's own attempts.

`INTEGRITY_DATABASE_URL`, `KAFKA_BROKERS`, `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY` and `MINIO_SECRET_KEY` configure the worker. The MinIO account needs read access only to extracted text in the submissions bucket. TLS defaults on. Set `MINIO_TLS=false` only on the private local network. `INTEGRITY_SCOPE=assignment` is the default. `course` also permits comparisons across assignments of the same course. An optional `INTEGRITY_TEMPLATES` JSON file maps assignment IDs to exact boilerplate text. When the template configuration or scope changes, replay into a fresh database. Database credentials require schema creation rights in the worker's dedicated database.

The input boundary is one immutable object per submission, at most 64 KiB of decoded UTF-8 bytes. The event cannot choose a host or another submission's object. Missing objects are retried. The index carries a content digest so subsequent changes are detectable. Empty, short or all-template documents abstain. Exact verification uses validation Jaccard 0.80 and a minimum 50-token run from the seed-42 PAN study. Matching code-point spans refer to the original decoded string, including line breaks. At most twenty longest spans per pair are retained. PAN pair recall was zero on the evaluated test subsets. This service can supply literal-overlap evidence for review, with limited coverage of paraphrase and fragmented reuse.

The publisher records success after a broker acknowledgement. A crash can redeliver the same event ID. Database and Kafka access must be restricted to trusted producers and services. A candidate whose underlying object is unavailable can hold the current comparison pending. Source content is read on demand, and untrusted text is never executed.

```sh
docker build -f ml/integrity_service/Dockerfile -t examory/text-integrity:local .
python -m unittest ml.integrity_service.test_service -v
```

Set `TEST_INTEGRITY_DATABASE_URL` to run the durable lifecycle test on a disposable PostgreSQL database. The fixture exercises delayed object availability, duplicate deliveries, assignment isolation and acknowledgement failure. Unit cases cover exact overlap, one modified word, template exclusion, independent and short submissions.
