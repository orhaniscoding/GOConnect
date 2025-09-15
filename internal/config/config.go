package config

import (
    "errors"
    "os"
    "path/filepath"
    "strings"

    yaml "gopkg.in/yaml.v3"
)

type Config struct {
    Port          int      `yaml:"port"`
    MTU           int      `yaml:"mtu"`
    LogLevel      string   `yaml:"log_level"`
    Language      string   `yaml:"language"`
    Autostart     bool     `yaml:"autostart"`
    ControllerURL string   `yaml:"controller_url"`
    RelayURLs     []string `yaml:"relay_urls"`
    UDPPort       int      `yaml:"udp_port"`
    Peers         []string `yaml:"peers"`
}

const (
    DefaultPort     = 2537
    DefaultMTU      = 1280
    DefaultLogLevel = "info"
)

func Default(language string) *Config {
    return &Config{
        Port:      DefaultPort,
        MTU:       DefaultMTU,
        LogLevel:  DefaultLogLevel,
        Language:  language,
        Autostart: true,
        RelayURLs: []string{},
        UDPPort:   45820,
        Peers:     []string{},
    }
}

func ProgramDataBase() string {
    pd := os.Getenv("ProgramData")
    if pd == "" {
        // Fallback for environments where ProgramData isn't set
        pd = `C:\\ProgramData`
    }
    return filepath.Join(pd, "GOConnect")
}

func Paths() (configDir, logDir, secretsDir string) {
    base := ProgramDataBase()
    return filepath.Join(base, "config"), filepath.Join(base, "logs"), filepath.Join(base, "secrets")
}

func ConfigFilePath() string {
    cfgDir, _, _ := Paths()
    return filepath.Join(cfgDir, "config.yaml")
}

func EnsureDirs() error {
    cfg, logs, secrets := Paths()
    for _, d := range []string{cfg, logs, secrets} {
        if err := os.MkdirAll(d, 0o755); err != nil {
            return err
        }
    }
    return nil
}

// Load reads a minimal YAML format we control.
func Load() (*Config, error) {
    if err := EnsureDirs(); err != nil {
        return nil, err
    }
    path := ConfigFilePath()
    if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
        // derive system locale as default
        lang := systemLocale()
        cfg := Default(lang)
        if err := Save(cfg); err != nil {
            return cfg, nil // still usable in-memory
        }
        return cfg, nil
    }
    b, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    cfg := Default(systemLocale())
    if err := yaml.Unmarshal(b, cfg); err != nil {
        return cfg, nil // tolerans: bozuk ise varsayılanlarla devam
    }
    return cfg, nil
}

func Save(cfg *Config) error {
    if err := EnsureDirs(); err != nil {
        return err
    }
    path := ConfigFilePath()
    out, err := yaml.Marshal(cfg)
    if err != nil { return err }
    return os.WriteFile(path, out, 0o644)
}

func systemLocale() string {
    // Very simple heuristic: check LANG or fallback to en.
    // In Windows, reading registry would be more accurate; TODO v1.3: improve.
    if l := os.Getenv("LANG"); strings.HasPrefix(strings.ToLower(l), "tr") {
        return "tr"
    }
    return "en"
}
