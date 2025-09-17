//go:build windows
// +build windows

package main

import (
	"embed"
)

//go:embed i18n/en.json i18n/tr.json
var i18nFS embed.FS

// ...existing code...
