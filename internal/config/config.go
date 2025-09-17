package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

type Network struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Joined      bool   `yaml:"joined"`
	Address     string `yaml:"address"`
	JoinSecret  string `yaml:"join_secret,omitempty"`
}

type Config struct {
	Port             int       `yaml:"port"`
	MTU              int       `yaml:"mtu"`
	LogLevel         string    `yaml:"log_level"`
	Language         string    `yaml:"language"`
	Autostart        bool      `yaml:"autostart"`
	ControllerURL    string    `yaml:"controller_url"`
	RelayURLs        []string  `yaml:"relay_urls"`
	UDPPort          int       `yaml:"udp_port"`
	Peers            []string  `yaml:"peers"`
	StunServers      []string  `yaml:"stun_servers"`
	TrustedPeerCerts []string  `yaml:"trusted_peer_certs"`
	Networks         []Network `yaml:"networks"`
}

const (
	DefaultPort     = 2537
	DefaultMTU      = 1280
	DefaultLogLevel = "info"
)

func Default(language string) *Config {
	return &Config{
		Port:             DefaultPort,
		MTU:              DefaultMTU,
		LogLevel:         DefaultLogLevel,
		Language:         language,
		Autostart:        true,
		RelayURLs:        []string{},
		UDPPort:          45820,
		Peers:            []string{},
		StunServers:      []string{"stun.l.google.com:19302"},
		TrustedPeerCerts: []string{},
		Networks:         []Network{},
	}
}

func ProgramDataBase() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		// Fallback for non-Windows or unusual environments
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

func Load() (*Config, error) {
	if err := EnsureDirs(); err != nil {
		return nil, err
	}
	path := ConfigFilePath()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		lang := systemLocale()
		cfg := Default(lang)
		if err := Save(cfg); err != nil {
			return cfg, nil
		}
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := Default(systemLocale())
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("config parse: %w", err)
	}
	if cfg.Networks == nil {
		cfg.Networks = []Network{}
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if cfg.MTU == 0 {
		cfg.MTU = DefaultMTU
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = DefaultLogLevel
	}
	if cfg.TrustedPeerCerts == nil {
		cfg.TrustedPeerCerts = []string{}
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	path := ConfigFilePath()
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func systemLocale() string {
	if l := os.Getenv("LANG"); strings.HasPrefix(strings.ToLower(l), "tr") {
		return "tr"
	}
	return "en"
}
