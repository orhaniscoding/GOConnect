package webuiassets

import (
	"embed"
)

// Embed required webui assets (recursive for i18n and all JS modules)
//
//go:embed index.html styles.css app.js *.js i18n/*
var FS embed.FS
