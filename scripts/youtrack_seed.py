#!/usr/bin/env python3
"""Create the LMS project and stage-1 tasks in YouTrack.

Usage:
  export YOUTRACK_URL=https://<name>.youtrack.cloud
  export YOUTRACK_TOKEN=perm:...        # Profile -> Account Security -> New token (scope: YouTrack)
  python3 scripts/youtrack_seed.py [--assignees aitbek=login1,syrym=login2,yerdaulet=login3] [--dry-run]

Re-running is safe: issues whose summary already exists in the project are skipped.
"""
import argparse, json, os, sys, urllib.request, urllib.parse, urllib.error

PROJECT = {"name": "Examory LMS", "shortName": "LMS"}

# (summary, owner key, done?, description)
ISSUES = [
    ("Set up repository, structure, README and CI", "aitbek", True,
     "GitHub repo Newterios/Examory: folder structure, README (team, goal, stack, stages, workflow), "
     "GitHub Actions CI, docker-compose with PostgreSQL/MongoDB/Redis/Kafka/MinIO."),
    ("Microservice architecture document", "aitbek", True,
     "docs/03_architecture/architecture.md: principles, context diagram, list of services with responsibilities "
     "and stores, data flow example, infrastructure, open questions."),
    ("Learning-activity event contract for Kafka", "aitbek", True,
     "docs/03_architecture/events.md: envelope, topics, payload of every platform and AI event, versioning rules. "
     "Shared contract for analytics and ML modules."),
    ("Local infrastructure in Docker Compose", "aitbek", True,
     "docker-compose.yml with PostgreSQL 16, MongoDB 7, Redis 7, Kafka 3.8 (KRaft), Kafka UI, MinIO. Validated in CI."),
    ("Functional requirements and technical specification draft", "syrym", False,
     "docs/02_requirements/technical_specification.md and glossary.md: purpose, users and roles, functional "
     "requirements per module (LMS core, integrity, analytics, anomaly detection, proctoring), non-functional "
     "requirements, constraints (privacy, human-in-the-loop). Deliverable: PR reviewed by the team lead."),
    ("UML: use case, data model and sequence diagrams", "syrym", False,
     "docs/04_uml: use case diagram (student, teacher, admin); domain data model; sequence diagrams for "
     "assignment submission with integrity check, quiz attempt with anomaly detection, exam session with proctoring. "
     "PlantUML source + exported PNG."),
    ("Figma prototypes of student and teacher dashboards", "syrym", False,
     "Student dashboard (progress, weak topics, recommendations) and teacher dashboard (group performance, "
     "at-risk students, risk signals on a submission). Link to Figma and PNG exports in docs/04_uml/prototypes/."),
    ("Literature review: plagiarism and AI-text detection", "yerdaulet", False,
     "docs/05_literature/plagiarism_detection.md: shingles/MinHash, embeddings, code token similarity, "
     "AI-generated text indicators. 10-15 sources with BibTeX in sources.bib. Comparison table of methods."),
    ("Literature review: behavioral anomaly detection and proctoring", "yerdaulet", False,
     "docs/05_literature/anomaly_detection.md: Isolation Forest, autoencoders, time-pattern features, "
     "synchronous submissions; online proctoring (presence, gaze, virtual camera, deepfake). 10-15 sources."),
    ("Dataset selection and baseline experiment plan", "yerdaulet", False,
     "ml/notebooks/datasets.md: candidate open datasets for plagiarism (text and code) and for quiz behavior; "
     "plan of a synthetic data generator for anomalies; evaluation protocol (precision, recall, false-positive rate)."),
    ("Stage report pre-defense-1 and tag", "aitbek", False,
     "Update docs/06_reports/pre-defense-1.md with links to merged PRs, create tag pre-defense-1, fill the "
     "probationary-period form."),
    # stage 2 backlog
    ("auth service: JWT, refresh tokens in Redis, RBAC", "aitbek", False, "services/auth, Go/Gin. Stage 2."),
    ("users and courses services", "aitbek", False, "services/users, services/courses with Kafka producers. Stage 2."),
    ("assignments and quizzes services with event publishing", "aitbek", False,
     "services/assignments, services/quizzes publishing submission.* and quiz.* events per events.md. Stage 2."),
    ("Analytics module skeleton and Kafka consumers", "syrym", False,
     "analytics/: Express service consuming lms.quizzes and lms.submissions, aggregation in MongoDB. Stage 2."),
    ("Metric definitions and dashboard prototypes (Chart.js)", "syrym", False, "Stage 2."),
    ("Baseline plagiarism detector (MinHash) with evaluation", "yerdaulet", False, "ml/integrity. Stage 2."),
    ("Baseline quiz-behavior anomaly detector (Isolation Forest)", "yerdaulet", False, "ml/anomaly. Stage 2."),
    ("Synthetic data generator for quiz behavior", "yerdaulet", False, "ml/datasets. Stage 2."),
]


def api(method, path, body=None, token=None, base=None):
    req = urllib.request.Request(base + path, method=method,
                                 data=json.dumps(body).encode() if body is not None else None)
    req.add_header("Authorization", "Bearer " + token)
    req.add_header("Accept", "application/json")
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        sys.exit(f"{method} {path} -> {e.code}: {e.read().decode()[:500]}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--assignees", default="", help="aitbek=login,syrym=login,yerdaulet=login")
    ap.add_argument("--dry-run", action="store_true")
    a = ap.parse_args()
    base, token = os.environ.get("YOUTRACK_URL", "").rstrip("/"), os.environ.get("YOUTRACK_TOKEN", "")
    if not a.dry_run and (not base or not token):
        sys.exit("Set YOUTRACK_URL and YOUTRACK_TOKEN")
    logins = dict(p.split("=", 1) for p in a.assignees.split(",") if "=" in p)

    if a.dry_run:
        for i, (s, who, done, _) in enumerate(ISSUES, 1):
            print(f"LMS-{i:<3} [{'done' if done else 'open'}] {who:<9} {s}")
        return

    me = api("GET", "/api/users/me?fields=id,login", token=token, base=base)
    projects = api("GET", "/api/admin/projects?fields=id,shortName", token=token, base=base)
    proj = next((p for p in projects if p["shortName"] == PROJECT["shortName"]), None)
    if proj is None:
        proj = api("POST", "/api/admin/projects?fields=id,shortName",
                   {**PROJECT, "leader": {"id": me["id"]}}, token=token, base=base)
        print("created project", proj["shortName"])
    else:
        print("project exists", proj["shortName"])

    existing = {i["summary"] for i in api(
        "GET", f"/api/issues?query={urllib.parse.quote('project: ' + PROJECT['shortName'])}&fields=summary&$top=500",
        token=token, base=base)}

    for summary, who, done, desc in ISSUES:
        if summary in existing:
            print("skip   ", summary); continue
        fields = []
        if who in logins:
            fields.append({"name": "Assignee", "$type": "SingleUserIssueCustomField", "value": {"login": logins[who]}})
        if done:
            fields.append({"name": "State", "$type": "StateIssueCustomField", "value": {"name": "Done"}})
        body = {"project": {"id": proj["id"]}, "summary": summary, "description": desc, "customFields": fields}
        issue = api("POST", "/api/issues?fields=idReadable", body, token=token, base=base)
        print("created", issue["idReadable"], summary)


if __name__ == "__main__":
    main()
