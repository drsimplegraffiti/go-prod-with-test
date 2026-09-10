.PHONY: run build test test-cover vet fmt lint docker-up docker-down migrate-up tidy

APP_NAME := goapi
BIN_DIR := bin

run: ## Run the API locally (requires DATABASE_URL etc. in env or .env)
	go run ./cmd/api

build: ## Compile the server binary
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(APP_NAME) ./cmd/api

test: ## Run the full test suite
	go test ./... -race -count=1

test-cover: ## Run tests with a coverage report
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

vet: ## Run go vet
	go vet ./...

fmt: ## Check formatting
	gofmt -l .

tidy: ## Tidy go.mod/go.sum
	go mod tidy

docker-up: ## Start Postgres + API via docker compose
	docker compose up --build

docker-down: ## Tear down docker compose stack
	docker compose down -v

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
