.PHONY: install-goose

LOCAL_BIN:=$(CURDIR)/bin
MIGRATION_NAME ?= init.sql

install-goose:
	$(info Installing goose binary into [$(LOCAL_BIN)]...)
	GOBIN=$(LOCAL_BIN) go install github.com/pressly/goose/v3/cmd/goose@v3.24.1

install-sqlc:
	$(info Installing sqlc binary into [$(LOCAL_BIN)]...)
	GOBIN=$(LOCAL_BIN) go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0

install-swag:
	$(info Installing swag binary into [$(LOCAL_BIN)]...)
	GOBIN=$(LOCAL_BIN) go install github.com/swaggo/swag/cmd/swag@latest

create-migration-file:
	$(LOCAL_BIN)/goose -dir migrations create -s $(MIGRATION_NAME) sql

up-migrations-bin:
	$(LOCAL_BIN)/goose -dir migrations postgres "postgresql://user:1234@127.0.0.1:5432/ozon_simulator_go_products?sslmode=disable" up

up-migrations:
	goose -dir migrations postgres "postgresql://postgres:1234@127.0.0.1:5432/ozon_simulator_go_products?sslmode=disable" up

compile-sql-bin:
	$(LOCAL_BIN)/sqlc generate

generate-swagger-bin:
	$(LOCAL_BIN)/swag init -g cmd/server/main.go -o internal/infra/swagger --parseDependency --parseInternal

proto-generate:
	protoc \
      -I . \
      -I C:/Git/googleapis \
      -I C:/Git/grpc-gateway \
      -I C:/Git/protoc/include \
      --go_out=./internal/app/gen \
      --go-grpc_out=./internal/app/gen \
      --grpc-gateway_out=./internal/app/gen \
      --openapiv2_out=swagger \
      api/v1/products.proto