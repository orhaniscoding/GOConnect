package webuiassets

import (
    "embed"
)

// Embed required webui assets (recursive for i18n)
//go:embed index.html app.js styles.css i18n/*
var FS embed.FS
