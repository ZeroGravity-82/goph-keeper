MODULE := zerogravity-82/goph-keeper

# DSN по умолчанию для локальных интеграционных тестов (compose.test.yaml).
TEST_DATABASE_URI ?= postgres://gophkeeper:userpassword@localhost:15432/gophkeeper_test?sslmode=disable

.PHONY: help fmt test proto

help: ## Показать доступные цели
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "%-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Отформатировать Go-файлы
	gofmt -w ./cmd ./internal ./migrations

test: ## Запустить unit-тесты
	go test ./...

proto: ## Перегенерировать Go-код из .proto-файлов
	rm -f internal/pb/*.pb.go
	protoc --go_out=. \
	  --go_opt=module=$(MODULE) \
	  --go-grpc_out=. \
	  --go-grpc_opt=module=$(MODULE) \
	  --go_opt=default_api_level=API_OPAQUE \
	  api/*.proto
