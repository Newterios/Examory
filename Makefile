# Examory developer entry points. Go modules live under services/.
GO_MODULES := services/files services/assignments services/platform services/auth services/courses services/quizzes services/grading
GOBIN ?= $(HOME)/go/bin
PYTHON ?= python3
MODEL_DIR := ml/service/model
ANOMALY_IMAGE := examory/anomaly-scorer:dev
ANOMALY_STUDENTS ?= 2000

.PHONY: dev-env infra up down vet fmt test test-race integration vuln check anomaly-model sync-schemas py-test py-lint m4 e2e

dev-env: ## generate .env with random development secrets
	./scripts/dev-env.sh

infra: ## start infrastructure only (postgres, redis, kafka)
	docker compose up -d postgres postgres-init redis kafka kafka-init

up: ## build and start the whole stack (gateway on 127.0.0.1:8000)
	docker compose up -d --build

down:
	docker compose down

vet:
	@for m in $(GO_MODULES); do echo "== $$m"; (cd $$m && go vet ./... && go vet -tags integration ./...) || exit 1; done

fmt:
	@out=$$(gofmt -l services); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

test-race:
	@for m in $(GO_MODULES); do echo "== $$m"; (cd $$m && go test -race ./...) || exit 1; done

test: test-race

integration: ## needs `make infra`; reads credentials from .env
	@set -a; . ./.env; set +a; \
	export IT_PG_ADMIN_URL="postgres://examory:$$POSTGRES_PASSWORD@127.0.0.1:5432/postgres?sslmode=disable" \
	       IT_REDIS_ADDR=127.0.0.1:6379 IT_REDIS_PASSWORD="$$REDIS_PASSWORD" IT_KAFKA_BROKERS=127.0.0.1:9092; \
	for m in $(GO_MODULES); do echo "== $$m"; (cd $$m && go test -race -count=1 -tags integration ./...) || exit 1; done

vuln:
	@for m in $(GO_MODULES); do echo "== $$m"; (cd $$m && $(GOBIN)/govulncheck ./...) || exit 1; done

check: fmt vet test-race vuln

anomaly-model: ## train the demo anomaly model INSIDE the scorer image (same library versions as runtime)
	docker build -f ml/service/Dockerfile -t $(ANOMALY_IMAGE) .
	mkdir -p $(MODEL_DIR)
	docker run --rm --read-only --tmpfs /tmp --user $$(id -u):$$(id -g) -v "$(CURDIR)/$(MODEL_DIR):/out" \
	  --entrypoint sh $(ANOMALY_IMAGE) -c 'set -e; \
	    python ml/datasets/generate_quiz_behavior.py --students $(ANOMALY_STUDENTS) --seed 42 --output /out/quiz.csv; \
	    python -m ml.anomaly.split --csv /out/quiz.csv --seed 42 --output /out/split.json; \
	    python -m ml.anomaly.train --csv /out/quiz.csv --manifest /out/split.json --output /out'
	chmod 644 $(MODEL_DIR)/model.pkl
	@sha=$$(shasum -a 256 $(MODEL_DIR)/model.pkl | cut -d' ' -f1); echo "$$sha" > $(MODEL_DIR)/model.sha256; \
	if [ -f .env ]; then grep -v '^ANOMALY_MODEL_SHA256=' .env > .env.tmp; echo "ANOMALY_MODEL_SHA256=$$sha" >> .env.tmp; \
	  mv .env.tmp .env; chmod 600 .env; fi; echo "model.pkl sha256=$$sha (pinned in .env as ANOMALY_MODEL_SHA256)"

sync-schemas: ## refresh the schema copies embedded by grading
	cp docs/03_architecture/schemas/envelope.schema.json docs/03_architecture/schemas/risk.behavior_anomaly.v1.schema.json \
	  services/grading/internal/adapters/schemas/

py-test: ## anomaly scorer unit tests (needs ml/service/requirements.txt installed)
	$(PYTHON) -m unittest discover -s ml/service/tests -v

py-lint:
	$(PYTHON) -m ruff check ml analytics

m4: ## bring the whole M4 scenario up (secrets, model, images, stack)
	./scripts/dev-env.sh
	$(MAKE) anomaly-model
	docker compose -f docker-compose.yml -f docker-compose.research.yml up -d --build --wait auth courses quizzes grading anomaly-scorer gateway

e2e: ## M4 end-to-end test through the gateway (needs `make m4`, which enables the research overlay)
	@set -a; . ./.env; set +a; cd services/grading && \
	E2E_GATEWAY_URL=http://127.0.0.1:8000 E2E_QUIZZES_URL=http://127.0.0.1:8001 E2E_ADMIN_EMAIL="$$AUTH_BOOTSTRAP_ADMIN_EMAIL" \
	E2E_ADMIN_PASSWORD="$$AUTH_BOOTSTRAP_ADMIN_PASSWORD" IT_KAFKA_BROKERS=127.0.0.1:9092 \
	IT_PG_ADMIN_URL="postgres://examory:$$POSTGRES_PASSWORD@127.0.0.1:5432/postgres?sslmode=disable" \
	go test -count=1 -tags integration -run M4 -v ./integration/
