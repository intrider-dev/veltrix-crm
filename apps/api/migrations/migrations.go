package migrations

import "embed"

// Files contains ordered SQL migrations embedded into the production binary.
//
//go:embed *.up.sql
var Files embed.FS
