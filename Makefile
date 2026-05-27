.PHONY: build build-server build-daemon build-web test dev daemon check clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

# Build all components
build: build-server build-daemon build-web

build-server:
	cd server && go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o bin/server ./cmd/server

build-daemon:
	cd daemon && go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o bin/af ./cmd/af

build-web:
	cd web && npm run build

# Testing
test:
	cd server && go test ./...
	cd daemon && go test ./...
	cd shared && go test ./...
	cd web && npm test -- --run 2>/dev/null || true

# Development
dev:
	cd server && go run ./cmd/server

daemon:
	cd daemon && go run ./cmd/af daemon start --foreground

# Verification pipeline
check:
	cd server && go vet ./... && go build ./... && go test ./...
	cd daemon && go vet ./... && go build ./... && go test ./...
	cd shared && go vet ./... && go build ./... && go test ./...

clean:
	rm -rf server/bin daemon/bin web/dist
