.PHONY: build run test test-race vet lint docker-up docker-down

# Application binary name
BINARY_NAME=quickcart-server

build:
	go build -o bin/$(BINARY_NAME) ./cmd/server

run:
	go run ./cmd/server

test:
	go test -v ./...

test-race:
	go test -v -race ./...

vet:
	go vet ./...

docker-up:
	docker compose -f deployments/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker-compose.yml down
