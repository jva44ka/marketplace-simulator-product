.PHONY: install-goose

up-migrations:
	goose -dir migrations postgres "postgresql://postgres:1234@127.0.0.1:5432/ozon_simulator_go_products?sslmode=disable" up

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

docker-build-latest:
	docker build -t jva44ka/ozon-simulator-go-products:latest .
docker-push-latest:
	docker push jva44ka/ozon-simulator-go-products:latest

docker-build-migrator:
	docker build --target migrator -t jva44ka/ozon-simulator-go-products:migrator .
docker-push-migrator:
	docker push jva44ka/ozon-simulator-go-products:migrator