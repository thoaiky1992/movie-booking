// Package migrations embeds the goose SQL migrations so the migrate binary
// and the container image carry their schema with them.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
