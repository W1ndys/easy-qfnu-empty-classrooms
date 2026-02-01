package web

import "embed"

//go:embed index.html css images
var StaticFS embed.FS
