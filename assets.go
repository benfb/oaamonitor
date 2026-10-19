// Package oaamonitor embeds the templates and static assets so the binaries
// no longer depend on the working directory.
package oaamonitor

import (
	"embed"
	"io/fs"
)

//go:embed templates static
var assets embed.FS

// Templates returns the HTML templates, rooted at the templates directory.
func Templates() fs.FS {
	sub, err := fs.Sub(assets, "templates")
	if err != nil {
		panic(err)
	}
	return sub
}

// Static returns the static assets, rooted at the static directory.
func Static() fs.FS {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
