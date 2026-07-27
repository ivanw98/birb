// Package migrations embeds the goose SQL migrations so the API binary can apply them without the goose CLI.
package migrations

import "embed"

// FS holds every numbered migration, rooted at the FS top level.
//
//go:embed *.sql
var FS embed.FS
