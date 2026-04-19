.PHONY: install-goose

up-migrations:
	goose -dir migrations postgres "postgresql://postgres:1234@127.0.0.1:5432/marketplace_simulator_product?sslmode=disable" up

proto-generate:
	protoc \
      -I . \
      -I C:/Git/googleapis \
      -I C:/Git/grpc-gateway \
      -I C:/Git/protoc/include \
      --go_out=./internal/app/pb \
      --go-grpc_out=./internal/app/pb \
      --grpc-gateway_out=./internal/app/pb \
      --openapiv2_out=swagger \
      api/v1/products.proto

docker-build-latest:
	docker build -t jva44ka/marketplace-simulator-product:latest .
docker-push-latest:
	docker push jva44ka/marketplace-simulator-product:latest

docker-build-migrator:
	docker build --target migrator -t jva44ka/marketplace-simulator-product:migrator .
docker-push-migrator:
	docker push jva44ka/marketplace-simulator-product:migrator