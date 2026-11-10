# Project movie-booking

One Paragraph of project description goes here

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.

## MakeFile

Run build make command with tests
```bash
make all
```

Build the application
```bash
make build
```

Run the application
```bash
make run
```
Create DB container
```bash
make docker-run
```

Shutdown DB Container
```bash
make docker-down
```

DB Integrations Test:
```bash
make itest
```

## Database migrations

The project uses [Goose](https://github.com/pressly/goose) for PostgreSQL
migrations. Migration files live in `migrations/`. The Makefile reads the
existing `BLUEPRINT_DB_*` values from `.env`, so start PostgreSQL before
applying migrations:

```bash
make docker-run
make migration-up
```

Create a migration:

```bash
make migration-create name=create_movies
```

Check migration status or roll back the latest migration:

```bash
make migration-status
make migration-down
```

Goose is pinned as a Go tool in `go.mod`; Go downloads it automatically on the
first use. To use an installed binary instead, run a command with `GOOSE=goose`,
for example:

```bash
make migration-status GOOSE=goose
```

Live reload the application:
```bash
make watch
```

Run the test suite:
```bash
make test
```

Clean up binary from the last build:
```bash
make clean
```
