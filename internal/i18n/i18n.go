package i18n

import (
    "encoding/json"
    "os"
    "path/filepath"
    "sync"
)

var (
    mu     sync.RWMutex
    active = "en"
    dicts  = map[string]map[string]string{}
)

func LoadFromFiles(baseDir string) error {
    mu.Lock()
    defer mu.Unlock()
    dicts = map[string]map[string]string{}
    for _, lang := range []string{"en", "tr"} {
        p := filepath.Join(baseDir, lang+".json")
        b, err := os.ReadFile(p)
        if err != nil {
            dicts[lang] = map[string]string{}
            continue
        }
        m := map[string]string{}
        _ = json.Unmarshal(b, &m)
        dicts[lang] = m
    }
    return nil
}

func SetActiveLanguage(lang string) { mu.Lock(); active = lang; mu.Unlock() }
func ActiveLanguage() string        { mu.RLock(); defer mu.RUnlock(); return active }

func T(key string) string {
    mu.RLock()
    defer mu.RUnlock()
    if d, ok := dicts[active]; ok {
        if v, ok := d[key]; ok {
            return v
        }
    }
    if d, ok := dicts["en"]; ok {
        if v, ok := d[key]; ok {
            return v
        }
    }
    return key
}

