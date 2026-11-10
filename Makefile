# Simple Makefile for a Go project

GOOSE ?= go tool goose
GOOSE_DRIVER ?= postgres
GOOSE_MIGRATION_DIR ?= migrations
DATABASE_URL ?= postgres://$(BLUEPRINT_DB_USERNAME):$(BLUEPRINT_DB_PASSWORD)@$(BLUEPRINT_DB_HOST):$(BLUEPRINT_DB_PORT)/$(BLUEPRINT_DB_DATABASE)?sslmode=disable&search_path=$(BLUEPRINT_DB_SCHEMA)

-include .env

# Build the application
all: build test

build:
	@echo "Building..."
	
	
	@go build -o main cmd/api/main.go

# Run the application
run:
	@go run cmd/api/main.go
# Create DB container
docker-run:
	@if docker compose up --build 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose up --build; \
	fi

# Shutdown DB container
docker-down:
	@if docker compose down 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose down; \
	fi

# Test the application
test:
	@echo "Testing..."
	@go test ./... -v
# Integrations Tests for the application
itest:
	@echo "Running integration tests..."
	@go test ./internal/database -v

# Create a new SQL migration: make migration-create name=create_movies
migration-create:
	@test -n "$(name)" || (echo "Usage: make migration-create name=create_movies" && exit 1)
	@$(GOOSE) -dir $(GOOSE_MIGRATION_DIR) create $(name) sql

# Apply all pending migrations
migration-up:
	@$(GOOSE) -dir $(GOOSE_MIGRATION_DIR) $(GOOSE_DRIVER) "$(DATABASE_URL)" up

# Roll back the latest migration
migration-down:
	@$(GOOSE) -dir $(GOOSE_MIGRATION_DIR) $(GOOSE_DRIVER) "$(DATABASE_URL)" down

# Show migration status
migration-status:
	@$(GOOSE) -dir $(GOOSE_MIGRATION_DIR) $(GOOSE_DRIVER) "$(DATABASE_URL)" status

# Clean the binary
clean:
	@echo "Cleaning..."
	@rm -f main

# Live Reload
watch:
	@if command -v air > /dev/null; then \
            air; \
            echo "Watching...";\
        else \
            read -p "Go's 'air' is not installed on your machine. Do you want to install it? [Y/n] " choice; \
            if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
                go install github.com/air-verse/air@latest; \
                air; \
                echo "Watching...";\
            else \
                echo "You chose not to install air. Exiting..."; \
                exit 1; \
            fi; \
        fi

.PHONY: all build run test clean watch docker-run docker-down itest migration-create migration-up migration-down migration-status
