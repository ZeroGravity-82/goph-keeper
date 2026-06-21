MODULE := zerogravity-82/goph-keeper
DC := docker compose

# DSN по умолчанию для локальных интеграционных тестов (compose.test.yaml).
TEST_DATABASE_URI ?= postgres://gophkeeper:userpassword@localhost:15432/gophkeeper_test?sslmode=disable

.PHONY: help fmt test lint up down proto db-test-up db-test-down test-integration

help: ## Показать доступные цели
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "%-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Отформатировать Go-файлы
	gofmt -w ./cmd ./internal ./migrations

test: ## Запустить unit-тесты
	go test ./...

lint: ## Запустить базовые статические проверки (go vet)
	go vet ./...

up: ## Запустить контейнеры Docker Composer
	$(DC) up -d

down: ## Остановить и удалить контейнеры Docker Composer
	$(DC) down

proto: ## Перегенерировать Go-код из .proto-файлов
	rm -f internal/pb/*.pb.go
	protoc --go_out=. \
	  --go_opt=module=$(MODULE) \
	  --go-grpc_out=. \
	  --go-grpc_opt=module=$(MODULE) \
	  --go_opt=default_api_level=API_OPAQUE \
	  api/*.proto

# --- Интеграционная БД (Docker Compose) ---

db-test-up: ## Запустить изолированный PostgreSQL для интеграционных тестов
	docker compose --project-name gophkeeper-itest -f compose.test.yaml up -d

db-test-down: ## Остановить и удалить контейнер с PostgreSQL для интеграционных тестов
	docker compose --project-name gophkeeper-itest -f compose.test.yaml down

# --- Интеграционные тесты ---

test-integration: db-test-up ## Запустить интеграционные тесты PostgreSQL
	TEST_DATABASE_URI='$(TEST_DATABASE_URI)' go test -p 1 -tags=integration ./internal/...
