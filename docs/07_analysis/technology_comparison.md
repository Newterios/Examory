# Technology comparison

Each key decision lists 2–3 alternatives, the criteria that matter for Examory, and the choice with the requirements it serves. Requirement IDs refer to `docs/02_requirements/technical_specification.md`.

## Summary of decisions

| # | Decision | Chosen | Alternatives considered | Main driver |
|---|----------|--------|-------------------------|-------------|
| D1 | Core service platform | Go (Gin) | Node.js (NestJS), Python (FastAPI) | NFR-1 latency, NFR-11 availability, team experience (EduLMS) |
| D2 | Event bus | Apache Kafka | RabbitMQ, NATS JetStream | NFR-12 replay and deduplication, 365-day retention of learning events |
| D3 | Data storage | PostgreSQL + MongoDB + Redis + MinIO | PostgreSQL only; MongoDB only | Relational integrity for grades (NFR-16) vs flexible aggregates for analytics |
| D4 | Synchronous service interaction | REST/JSON externally, gRPC internally | REST everywhere; events only | FR-9/FR-14 need immediate acknowledgements; typed contracts between Go services |
| D5 | ML inference serving | Python services (FastAPI) for all models in the MVP; Rust (Axum) + ONNX Runtime only as a *could* option if measurements require it | Rust/ONNX from the start; Triton / TorchServe | Target load (20 events/s, NFR-3 10 s) is within reach of a Python service; MinHash/LSH is not an ONNX model |
| D6 | Web client | Next.js (TypeScript) | React SPA (Vite), Vue/Nuxt | SSR for dashboards, team experience, Chart.js/Recharts ecosystem (NFR-14, NFR-15) |

## D1. Core service platform

| Criterion | Go (Gin) | Node.js (NestJS) | Python (FastAPI) |
|-----------|----------|------------------|------------------|
| p95 latency under 100 concurrent users (NFR-1 ≤ 500 ms) | Compiled, goroutines; lowest resource use per request | Good for I/O-bound; single-threaded CPU | Good I/O; CPU-bound work needs workers |
| Static typing and contract safety (gRPC, D4) | Native protobuf/gRPC tooling | TypeScript; gRPC via community packages | Type hints; gRPC via grpcio |
| Kafka client maturity | segmentio/kafka-go, confluent-kafka-go | kafkajs | confluent-kafka-python |
| Deployment footprint | Single static binary per service, small images | Node runtime per image | Python runtime per image |
| Team experience | Team lead built EduLMS (10 Go services) | Analytics owner uses Node | ML owner uses Python |
| Risk | Verbose error handling; fewer batteries included | Memory per instance higher | GIL for CPU-bound paths |

**Decision:** Go for the ten core services. Node.js remains the platform of the analytics module (owned by the analytics engineer, I/O-bound aggregation over Kafka), Python for ML training and most inference. Polyglot boundaries are explicit through the event contract, so each owner uses the stack they are productive in.

## D2. Event bus

| Criterion | Apache Kafka | RabbitMQ | NATS JetStream |
|-----------|--------------|----------|----------------|
| Replay of history for new consumers (NFR-12, re-training on past events) | Log-based, consumers re-read from any offset | Queue semantics; messages gone after ack unless streams plugin | Stream with replay, retention policies |
| Long retention (365 d for `lms.quizzes`, `ai.risk`) | Designed for it | Not designed for long retention | Supported but less mature tooling |
| Ordering per key (one student's events in order) | Partition key guarantees order | Per-queue only | Per-subject |
| Ecosystem and tooling (UI, schema registry, connectors) | Kafka UI, Schema Registry, Connect | Management UI | NATS CLI |
| Operational weight for a single VPS MVP | KRaft single node is acceptable; ~1 GB RAM | Light | Lightest |
| Team experience | Used in EduLMS | — | Used in EduLMS v2 (course project) |

**Decision:** Kafka. Replay and per-key ordering are required by the analytics deduplication rule (FR-39) and by ML re-processing; the event contract's retention table assumes a log, not a queue. NATS was the lighter option and remains the fallback if the VPS cannot sustain Kafka.

## D3. Data storage

| Criterion | PostgreSQL + MongoDB (+ Redis, MinIO) | PostgreSQL only | MongoDB only |
|-----------|----------------------------------------|-----------------|--------------|
| Transactional integrity of grades, rubrics, consent records (NFR-16 audit, FR-17) | PostgreSQL | Yes | Multi-document transactions exist but weaker tooling |
| Flexible, evolving aggregates for analytics and notifications | MongoDB | JSONB can cover it | Native |
| Refresh tokens, rate limits, sessions (FR-1, NFR-7) | Redis | Possible but misuse of PostgreSQL | Possible |
| Files, submissions, exam video with presigned URLs (NFR-6) | MinIO (S3 API) | Large objects in DB are a bad fit | GridFS, no presigned URLs |
| Operational complexity | Four systems | One | One + object store anyway |

**Decision:** polyglot persistence. The cost is four systems in Docker Compose, which is accepted because each is used for what it is designed for; the "database per service" rule in architecture.md keeps coupling low. If operations prove too heavy, MongoDB is the first candidate to fold into PostgreSQL JSONB.

## D4. Synchronous service interaction

| Criterion | REST externally + gRPC internally | REST everywhere | Events only (no sync calls) |
|-----------|-----------------------------------|-----------------|-----------------------------|
| Immediate acknowledgements (submission receipt FR-9, attempt finalization FR-14) | Yes | Yes | No: receipts would be eventually consistent |
| Typed, versioned contracts between Go services | protobuf | OpenAPI, looser | Event schemas only |
| Browser and agent compatibility | REST/JSON via Nginx | Same | Same |
| Debuggability | gRPC needs tooling (grpcurl) | curl | Kafka UI |

**Decision:** REST/JSON at the gateway for the web client and the Windows agent; gRPC between core services where a synchronous answer is needed (authorization checks, reference reads). Everything that does not need an immediate answer goes through Kafka. This is the open question §6 of architecture.md, now resolved.

## D5. ML inference serving

| Criterion | Python services + Rust/ONNX Runtime for hot paths | Python FastAPI only | Triton / TorchServe |
|-----------|---------------------------------------------------|---------------------|---------------------|
| Signal latency ≤ 10 s (NFR-3) and per-answer scoring volume (`quiz.answered` is the highest-rate topic) | ONNX Runtime in Rust gives low-latency, low-memory scoring | Adequate for batch, slower per request | Low latency, GPU-oriented |
| Model lifecycle: scikit-learn / PyTorch → ONNX | Export path documented; team member has done it | No export needed | Needs model repository and Docker image per version |
| Resource cost on a single VPS | Low | Medium | High (designed for GPU clusters) |
| Development speed for experiments | Python for training and evaluation stays | Fastest | Slowest |

**Decision (revised 23 Oct 2026 after supervisor review):** all ML modules are served from Python (FastAPI, scikit-learn, datasketch) in the MVP. The Rust/ONNX service is moved to *could* and will be built only if a measurement on the target VPS shows a Python service cannot meet the budget. The trigger figures: p95 scoring latency of `quiz.answered` events above 2 s, or sustained CPU above 50 % of one vCPU at 20 events/s with the full stack running (measurement task LMS-34). MinHash/LSH is not an ONNX model and stays in Python in any case.

### D3a. Analytics module runtime: Node.js vs Python-only

| Criterion | Node.js/Express (chosen) | Python/FastAPI |
|-----------|--------------------------|----------------|
| Shares types and API conventions with the Next.js web client (same TypeScript models for dashboard payloads) | Yes | No, separate models |
| Kafka consumer and MongoDB aggregation pipelines | kafkajs + MongoDB aggregation; no pandas needed at runtime | confluent-kafka + pandas |
| Owner's productivity (analytics engineer) | Primary stack | Secondary |
| Number of runtimes in the component | One (Node.js); pandas is used only in offline notebooks, not in the service | One |

**Decision (revised):** the analytics service is Node.js only; aggregations run as MongoDB pipelines, and pandas is confined to offline analysis notebooks. This removes the two-runtime component noted in the review. Total runtimes in production: Go (core), Node.js (web client and analytics), Python (ML). Rust is dropped from the MVP (D5).

## D6. Web client

| Criterion | Next.js (TypeScript) | React SPA (Vite) | Vue / Nuxt |
|-----------|----------------------|------------------|------------|
| Dashboards with charts (Chart.js / Recharts), accessibility checks (NFR-14) | Mature ecosystem | Same ecosystem | Smaller chart ecosystem |
| SSR for first paint ≤ 2 s on dashboards (NFR-2) | Built in | Client-only | Nuxt provides SSR |
| Team experience | Used in EduLMS | — | — |
| Routing, auth integration with JWT | App Router + middleware | Manual | Nuxt middleware |

**Decision:** Next.js. The analytics engineer prototypes dashboards with Chart.js inside the same application.

## Decisions deferred to stage 2

- Schema registry for Kafka (Confluent Schema Registry vs JSON Schema files in the repository): start with JSON Schema in `docs/03_architecture/schemas/`, revisit when a third producer appears.
- Retention of exam recordings: 7 days proposed in the technical specification, to be approved by the university before the pilot.
