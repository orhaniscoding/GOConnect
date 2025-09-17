package internal

import (
	"embed"
	"encoding/json"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

var Bundle *i18n.Bundle

func InitI18n(defaultLang string) {
	Bundle = i18n.NewBundle(language.English)
	Bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	// Load all locale files
	entries, _ := localeFS.ReadDir("locales")
	for _, entry := range entries {
		if !entry.IsDir() {
			data, _ := localeFS.ReadFile("locales/" + entry.Name())
			Bundle.ParseMessageFileBytes(data, entry.Name())
		}
	}
}

func NewLocalizer(lang string) func(string) string {
	localizer := i18n.NewLocalizer(Bundle, lang)
	return func(id string) string {
		msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: id})
		if err != nil {
			return id // Çeviri bulunamazsa anahtar döner
		}
		return msg
	}
}
