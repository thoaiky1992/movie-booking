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

## Catalog APIs

The read-only catalog endpoints are served under `/api/v1`:

```text
GET /api/v1/cinemas
GET /api/v1/cinemas/:cinemaID/movies
GET /api/v1/movies/:movieID
GET /api/v1/cinemas/:cinemaID/movies/:movieID/showtimes
GET /api/v1/showtimes/:showtimeID/seats
```

The seat-map endpoint returns seats grouped by row. Seat status is one of
`AVAILABLE`, `HELD`, or `BOOKED`; an expired hold is treated as available.
The movie-detail endpoint includes the `external_ratings` array.

All successful API responses use the same envelope:

```json
{
  "success": true,
  "data": {},
  "trace_id": "8bc0d82e5c33435eb1d9f4ecbd1f8682"
}
```

Errors use a separate, consistent envelope:

```json
{
  "success": false,
  "error": {
    "code": "SHOWTIME_NOT_FOUND",
    "message": "showtime not found"
  },
  "trace_id": "8bc0d82e5c33435eb1d9f4ecbd1f8682"
}
```

The API accepts an optional `X-Trace-ID` request header and always returns it
in both the response header and body. Access, error, and lifecycle logs are
written as JSON and include `trace_id` for request-scoped entries. Set
`LOG_LEVEL` to `debug`, `info` (default), `warn`, or `error`.

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
