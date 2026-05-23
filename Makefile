.PHONY: help build dev test migrate-up migrate-down sqlc-generate check clean

# Configuration
POSTGRES_DB ?= agenticflow
POSTGRES_USER ?= agenticflow
POSTGRES_PASSWORD ?= agenticflow
POSTGRES_PORT ?= 5432
PORT ?= 8080
DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

export

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

.DEFAULT_GOAL := help

##@ Help

help: ## Show available make targets
	@awk 'BEGIN {FS = ":.*## "; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\n"} \
		/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next} \
		/^[a-zA-Z0-9_.-]+:.*## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ Development

dev: ## Start the server in development mode
	cd server && go run ./cmd/server

daemon: ## Run the daemon in foreground mode
	cd server && go run ./cmd/af daemon start --foreground

cli: ## Run the af CLI with ARGS
	cd server && go run ./cmd/af $(ARGS)

##@ Build

build: ## Build the server and CLI binaries into server/bin
	cd server && go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o bin/server ./cmd/server
	cd server && go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o bin/af ./cmd/af

##@ Testing

test: ## Run all Go tests
	cd server && go test ./...

test-race: ## Run Go tests with race detector
	cd server && go test -race ./...

check: ## Run the full verification pipeline (build + test + vet)
	cd server && go vet ./...
	cd server && go build ./...
	cd server && go test ./...

##@ Database

migrate-up: ## Run database migrations up
	cd server && go run ./cmd/server migrate up

migrate-down: ## Roll back database migrations
	cd server && go run ./cmd/server migrate down

sqlc-generate: ## Regenerate sqlc type-safe query code
	cd server && sqlc generate

##@ Cleanup

clean: ## Remove generated binaries and temp files
	rm -rf server/bin server/tmp
