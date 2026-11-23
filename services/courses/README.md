# Courses

Courses owns curriculum, teacher assignments, group enrollment and material completion in PostgreSQL. Modules and materials are arrays in teaching order. A course revision covers the complete ordered curriculum and membership changes.

| Requirement | Implemented behavior |
|---|---|
| FR-3 | An administrator grants or revokes teacher and group membership. |
| FR-4 | A teacher creates a draft and manages courses to which they are assigned. |
| FR-5 | Text and HTTP(S) links have independent draft, published and archived states. Students see published courses, modules and materials in their currently enrolled group. |
| FR-6 | Each view emits an event. Completion is monotonic and unique per course, student and material. |

## API

All public routes use a bearer JWT. Prefix each path with `/api/courses/v1` through the gateway.

| Method and path | Access and result |
|---|---|
| POST /courses | Teacher or admin. Send `{"title":"Algorithms"}`. Returns the new draft (201). A teacher creating a course is assigned automatically. |
| GET /courses | Current accessible courses, ordered by ID. Accepts limit and cursor. |
| GET /courses/{id} | Visible course and ordered content. Foreign or unpublished student resources return 404. |
| PUT /courses/{id} | Assigned teacher or admin. Send the full course with current revision. A stale revision returns 409. |
| PUT or DELETE /courses/{id}/teachers/{user_id} | Admin. Add or revoke an assignment. Idempotent, 204. |
| PUT or DELETE /courses/{id}/groups/{group_id} | Admin. Enroll or remove a group. Idempotent, 204. |
| POST /courses/{id}/materials/{material_id}/views | Student. Send `{"view_duration_ms":45000,"completed":true}`. Returns current completion. |

A teacher can publish a course and two materials with this PUT body. Use the revision returned by the latest GET, including after enrollment. Omit IDs on new items and retain the IDs returned by the service on subsequent updates.

```json
{"revision":1,"title":"Algorithms","status":"published","modules":[
 {"title":"Searching","status":"published","materials":[
  {"title":"Lecture","status":"published","kind":"text","content":"Binary search requires sorted input."},
  {"title":"Exercises","status":"draft","kind":"link","content":"https://example.edu/exercises"}
 ]}
]}
```

Only internal callers with `Authorization: Bearer $COURSES_INTERNAL_TOKEN` may query membership. `GET /internal/v1/access` requires a course_id and exactly one teacher_id or group_id. It returns a boolean allowed. `GET /internal/v1/teacher-courses?teacher_id=…` lists assigned course IDs. The gateway exposes only the public v1 prefix.

## Events and operation

Course creation, edits, membership changes and material views write the business row and outbox row in one transaction. The relay publishes to `lms.courses`, keyed by course ID. Each course mutation increments revision, except an idempotently repeated membership operation. The v1 schemas for `course.created` and `course.updated` carry course_id, status and revision. `material.viewed` carries module_id, material_id, view_duration_ms and completed. A consumer computes progress using distinct completed student/material pairs.

From the repository root run `make dev-env && make infra && make integration`, then `docker compose up -d --build courses`. `make check` runs formatting, vet, race tests and vulnerability checks. The integration test starts with a fresh PostgreSQL database, verifies denied actions and reads schema-validated messages from Kafka.

## Known limitations

Files belong to the files service. Courses currently trusts the administrator's supplied teacher and group UUIDs. The student's group comes from the signed access token, so group transfers in auth take effect when a token is refreshed. Removing a group's enrollment takes effect on the next course query. Concurrent edits use whole-course optimistic revisions. The request size limit is 512 KiB.
