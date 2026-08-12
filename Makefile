GOOSE ?= goose
SWAG ?= $(shell which swag 2>/dev/null || echo $(shell go env GOPATH)/bin/swag)
WIRE ?= $(shell which wire 2>/dev/null || echo $(shell go env GOPATH)/bin/wire)
GOLANGCI_LINT ?= $(shell which golangci-lint 2>/dev/null || echo $(shell go env GOPATH)/bin/golangci-lint)
MIGRATION_DIR ?= ./migrations
ENV_FILE ?= .env

# Load .env if exists
ifneq (,$(wildcard $(ENV_FILE)))
	include $(ENV_FILE)
	export
endif

export GOOSE_DRIVER ?= postgres

# Bisa dioverride langsung dengan GOOSE_DBSTRING.
# Kalau tidak, build dari DB config existing kita.
export GOOSE_DBSTRING ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

MIGRATION_NAME = $(if $(name),$(name),$(NAME))

.PHONY: \
	goose-install \
	swagger-install \
	swagger-gen \
	migrate-create \
	migrate-up \
	migrate-down \
	migrate-status \
	migrate-redo \
	migrate-version \
	migrate-reset \
	test \
	test-integration \
	lint \
	lint-fix \
	wire-http \
	wire-mail \
	wire \
	run \
	run-http \
	run-worker \
	run-mail \
	build \
	docker-up \
	docker-down

goose-install:
	go install github.com/pressly/goose/v3/cmd/goose@latest

swagger-install:
	go install github.com/swaggo/swag/cmd/swag@latest

swagger-gen:
	$(SWAG) init -g cmd/http/main.go -o docs/swagger

migrate-create:
	@printf "Migration name: "; \
	read migration_name; \
	if [ -z "$$migration_name" ]; then \
		echo "Migration name cannot be empty"; \
		exit 1; \
	fi; \
	mkdir -p $(MIGRATION_DIR); $(GOOSE) -dir $(MIGRATION_DIR) create "$$migration_name" sql

migrate-up:
	$(GOOSE) -dir $(MIGRATION_DIR) up

migrate-down:
	$(GOOSE) -dir $(MIGRATION_DIR) down

migrate-status:
	$(GOOSE) -dir $(MIGRATION_DIR) status

migrate-redo:
	$(GOOSE) -dir $(MIGRATION_DIR) redo

migrate-version:
	$(GOOSE) -dir $(MIGRATION_DIR) version

migrate-reset:
	@printf "Reset ALL migrations and rebuild database? [y/N]: "; \
	read confirm; \
	if [ "$$confirm" != "y" ] && [ "$$confirm" != "Y" ]; then \
		echo "Cancelled"; \
		exit 0; \
	fi; \
	$(GOOSE) -dir $(MIGRATION_DIR) reset
	$(GOOSE) -dir $(MIGRATION_DIR) up

wire-http:
	$(WIRE) ./cmd/http

wire-mail:
	$(WIRE) ./cmd/worker/mail

wire: wire-http wire-mail

test-integration:
	go test -tags=integration -count=1 ./tests/integration/...

test:
	go test -count=1 ./...

lint:
	$(GOLANGCI_LINT) run ./...

lint-fix:
	$(GOLANGCI_LINT) run --fix ./...

run:
	@trap 'kill 0' INT TERM EXIT; \
	go run ./cmd/worker/mail & \
	go run ./cmd/http

run-http:
	go run ./cmd/http

run-worker:
	go run ./cmd/worker/mail

run-mail: run-worker

build:
	mkdir -p bin
	go build -o bin/http ./cmd/http
	go build -o bin/mail-worker ./cmd/worker/mail

docker-up:
	docker compose up -d

docker-down:
	docker compose down
