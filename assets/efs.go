package assets

import (
	"embed"
)

//go:embed "emails" "templates" "static" "build_egg.py" "migrations"
var EmbeddedFiles embed.FS
