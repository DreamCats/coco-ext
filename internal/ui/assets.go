package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedDistFS embed.FS

func embeddedStaticFS() (fs.FS, error) {
	return fs.Sub(embeddedDistFS, "dist")
}
