// Package flashpaper provides embedded static assets for the web interface.
// Using Go 1.16+ embed directive allows the entire application to be
// distributed as a single binary with no external dependencies.
package flashpaper

import (
	"embed"
	"io/fs"
)

// staticFiles embeds all static assets (JavaScript, CSS, images).
// These are served directly to clients without modification.
//
//go:embed web/static/*
var staticFiles embed.FS

// templateFiles embeds HTML templates for the web interface.
// Templates are processed by Go's html/template package before serving.
//
//go:embed web/templates/*
var templateFiles embed.FS

// StaticFS returns a filesystem containing static assets.
// The returned FS has the "web/static" prefix stripped for cleaner URLs.
// Example: web/static/js/flashpaper.js -> js/flashpaper.js
func StaticFS() (fs.FS, error) {
	return fs.Sub(staticFiles, "web/static")
}

// TemplateFS returns a filesystem containing HTML templates.
// The returned FS has the "web/templates" prefix stripped.
// Example: web/templates/index.html -> index.html
func TemplateFS() (fs.FS, error) {
	return fs.Sub(templateFiles, "web/templates")
}

// RawStaticFS returns the embedded static filesystem without path stripping.
// Useful when you need the full path context.
func RawStaticFS() embed.FS {
	return staticFiles
}

// RawTemplateFS returns the embedded template filesystem without path stripping.
// Useful when you need the full path context.
func RawTemplateFS() embed.FS {
	return templateFiles
}
