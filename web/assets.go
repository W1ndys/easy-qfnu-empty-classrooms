package web

import "embed"

//go:embed index.html empty-classroom.html full-day-status.html css images
var StaticFS embed.FS
