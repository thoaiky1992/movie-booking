# Movie Booking System — developer commands.

BIN := bin
GOFLAGS := -trimpath

.PHONY: all build run run-worker migrate-up migrate-down migrate-status seed \
        vet fmt clean watch docker-up docker-down docker-logs loadtest

all: vet build

build:
	@echo "Building binaries into $(BIN)/ ..."
	@mkdir -p $(BIN)
	@go build $(GOFLAGS) -o $(BIN)/api ./cmd/api
	@go build $(GOFLAGS) -o $(BIN)/worker ./cmd/worker
	@go build $(GOFLAGS) -o $(BIN)/migrate ./cmd/migrate
	@go build $(GOFLAGS) -o $(BIN)/seed ./cmd/seed
	@go build -o main ./cmd/api   # `main` is what .air.toml expects

run:
	@go run ./cmd/api

run-worker:
	@go run ./cmd/worker

migrate-up:
	@go run ./cmd/migrate up

migrate-down:
	@go run ./cmd/migrate down

migrate-status:
	@go run ./cmd/migrate status

seed:
	@go run ./cmd/seed

# Many customers race for one seat (needs the stack up and seed data loaded).
#   make loadtest USERS=500 BASE_URL=http://localhost:8080
BASE_URL ?= http://localhost:8080
USERS ?= 200
loadtest:
	@k6 run -e BASE_URL=$(BASE_URL) -e USERS=$(USERS) loadtest/seat_contention.js

vet:
	@go vet ./...

fmt:
	@gofmt -s -w cmd internal db

clean:
	@rm -rf $(BIN) main

watch:
	@if command -v air > /dev/null; then air; else echo "Install air: go install github.com/air-verse/air@latest"; exit 1; fi

docker-up:
	@docker compose up -d --build

docker-down:
	@docker compose down

docker-logs:
	@docker compose logs -f api worker
