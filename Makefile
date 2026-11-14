# Examory developer entry points. Go modules live under services/.
GO_MODULES := services/platform services/auth services/quizzes
GOBIN ?= $(HOME)/go/bin

.PHONY: dev-env infra up down vet fmt test test-race integration vuln check

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
