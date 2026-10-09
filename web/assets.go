package web

import "embed"

//go:embed templates/index.html static
var Assets embed.FS
