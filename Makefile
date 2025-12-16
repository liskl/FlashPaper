# FlashPaper Makefile
#
# Common commands for development, testing, and deployment.
#
# Usage:
#   make help      - Show this help message
#   make build     - Build the binary
#   make run       - Build and run locally
#   make test      - Run all tests
#   make docker    - Build Docker image
#   make up        - Start with docker-compose
#   make down      - Stop docker-compose

# Variables
BINARY_NAME := flashpaper
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Go settings
GO := go
GOFLAGS := CGO_ENABLED=1
GOTEST := $(GOFLAGS) $(GO) test
GOBUILD := $(GOFLAGS) $(GO) build

# Docker settings
DOCKER_IMAGE := flashpaper
DOCKER_TAG := latest

.PHONY: help build run test test-verbose test-coverage clean lint \
        docker docker-push up down dev logs

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo "FlashPaper - Zero-knowledge encrypted pastebin"
	@echo ""
	@echo "Usage:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/flashpaper/

## run: Build and run the application
run: build
	./$(BINARY_NAME)

## test: Run all tests
test:
	$(GOTEST) ./...

## test-verbose: Run all tests with verbose output
test-verbose:
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage report
test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-race: Run tests with race detector
test-race:
	$(GOTEST) -race ./...

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -rf tmp/
	rm -f *.db

## lint: Run linter (requires golangci-lint)
lint:
	golangci-lint run

## fmt: Format code
fmt:
	$(GO) fmt ./...

## mod-tidy: Clean up go.mod and go.sum
mod-tidy:
	$(GO) mod tidy

## mod-download: Download dependencies
mod-download:
	$(GO) mod download

## docker: Build Docker image
docker:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):$(VERSION)

## docker-push: Push Docker image to registry
docker-push: docker
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):$(VERSION)

## up: Start services with docker-compose
up:
	docker-compose up -d

## down: Stop docker-compose services
down:
	docker-compose down

## dev: Start development environment with hot reload
dev:
	docker-compose -f docker-compose.dev.yml up

## dev-build: Rebuild development container
dev-build:
	docker-compose -f docker-compose.dev.yml build

## logs: Show docker-compose logs
logs:
	docker-compose logs -f

## dev-logs: Show development logs
dev-logs:
	docker-compose -f docker-compose.dev.yml logs -f

## install-tools: Install development tools
install-tools:
	$(GO) install github.com/air-verse/air@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## db-shell: Open database shell (PostgreSQL)
db-shell:
	docker-compose exec db psql -U flashpaper -d flashpaper

## db-migrate: Run database migrations (if applicable)
db-migrate:
	@echo "No migrations to run - tables are auto-created on startup"

## version: Show version information
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
