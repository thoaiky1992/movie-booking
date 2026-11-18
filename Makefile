# Simple Makefile for a Go project

GOOSE ?= go tool goose
GOOSE_DRIVER ?= postgres
GOOSE_MIGRATION_DIR ?= database/migrations
DATABASE_URL ?= postgres://$(DB_USERNAME):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_DATABASE)?sslmode=disable&search_path=$(DB_SCHEMA)

-include .env

# Build the application
all: build test

build:
	@echo "Building..."
	
	
	@go build -o movie-booking .

# Run the application
run:
	@go run . server

# Run the seat-hold expiry worker as a separate process.
run-hold-expiry-worker:
	@go run . worker hold-expiry

# Backward-compatible alias.
run-worker: run-hold-expiry-worker

# Run the Asynq booking-confirmation email worker as a separate process.
run-email-worker:
	@go run . worker email

# Run the random external-rating updater as a separate process.
run-rating-updater-worker:
	@go run . worker rating-updater
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

# Contention test: requires SHOWTIME_ID, SEAT_IDS, and k6 on PATH.
loadtest-hot-seat:
	@command -v k6 >/dev/null || (echo "k6 is required: https://grafana.com/docs/k6/latest/set-up/install-k6/" && exit 1)
	@test -n "$(SHOWTIME_ID)" || (echo "SHOWTIME_ID is required" && exit 1)
	@test -n "$(SEAT_IDS)" || (echo "SEAT_IDS is required" && exit 1)
	@SHOWTIME_ID="$(SHOWTIME_ID)" SEAT_IDS="$(SEAT_IDS)" k6 run loadtests/k6/hot_seat_contention.js
# Integrations Tests for the application
itest:
	@echo "Running integration tests..."
	@go test ./modules/booking/infrastructure/persistence ./modules/catalog/infrastructure/persistence -v

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
	@rm -f movie-booking

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

.PHONY: all build run run-worker run-hold-expiry-worker run-email-worker run-rating-updater-worker test loadtest-hot-seat clean watch docker-run docker-down itest migration-create migration-up migration-down migration-status
