// Package migrations embeds the raw *.up.sql / *.down.sql files so they ship
// inside the compiled binary — no separate file deployment step needed.
package migrations

import "embed"

//go:embed files/*.sql
var FS embed.FS

// Dir is the directory name within FS that database.Migrate should read.
const Dir = "files"
