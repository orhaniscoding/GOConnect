package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	Port             int           `yaml:"port"`
	MTU              int           `yaml:"mtu"`
	LogLevel         string        `yaml:"log_level"`
	Language         string        `yaml:"language"`
	Autostart        bool          `yaml:"autostart"`
	EnableTun        bool          `yaml:"enable_tun"`
	ControllerURL    string        `yaml:"controller_url"`
	RelayURLs        []string      `yaml:"relay_urls"`
	UDPPort          int           `yaml:"udp_port"`
	Peers            []string      `yaml:"peers"`
	StunServers      []string      `yaml:"stun_servers"`
	TrustedPeerCerts []string      `yaml:"trusted_peer_certs"`
	Networks         []Network     `yaml:"networks"`
	Core             CoreConfig    `yaml:"core"`
	Api              ApiConfig     `yaml:"api"`
	Diag             DiagConfig    `yaml:"diag"`
	Updater          UpdaterConfig `yaml:"updater"`
	Logging          LoggingConfig `yaml:"logging"`
	Metrics          MetricsConfig `yaml:"metrics"`
}

type CoreConfig struct {
	BufferPackets   int           `yaml:"buffer_packets"`
	MaxFrameBytes   int           `yaml:"max_frame_bytes"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// ApiConfig controls local HTTP API security, rate limits, and validation.
type ApiConfig struct {
	Auth        string       `yaml:"auth"`         // "bearer" | "mtls" (future)
	BearerToken string       `yaml:"bearer_token"` // static owner token for bearer auth
	RateLimit   ApiRateLimit `yaml:"rate_limit"`   // requests per second limits
	Validation  bool         `yaml:"validation"`   // enable payload validation
}

type ApiRateLimit struct {
	RPS   int `yaml:"rps"`
	Burst int `yaml:"burst"`
}

// DiagConfig controls diagnostics probing parameters.
type DiagConfig struct {
	// MtuProbeMax is the upper bound used when probing link MTU.
	// Typical ethernet MTU is 1500; set higher only if your environment supports jumbo frames.
	MtuProbeMax int `yaml:"mtu_probe_max"`
}

// UpdaterConfig controls self-update behavior.
type UpdaterConfig struct {
	// Enabled toggles the updater feature.
	Enabled bool `yaml:"enabled"`
	// Repo is the GitHub repository in the form "owner/repo" to check releases from.
	Repo string `yaml:"repo"`
	// RequireSignature enforces signature verification for update artifacts when true.
	RequireSignature bool `yaml:"require_signature"`
	// PublicKey is the PEM-encoded public key used to verify signatures when RequireSignature is true.
	PublicKey string `yaml:"public_key"`
}

// LoggingConfig controls format and level of logs.
type LoggingConfig struct {
	Format string `yaml:"format"` // "json" | "text"
	Level  string `yaml:"level"`  // trace|debug|info|warn|error
}

// MetricsConfig controls optional Prometheus-style metrics endpoint.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
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
		EnableTun:        true,
		RelayURLs:        []string{},
		UDPPort:          45820,
		Peers:            []string{},
		StunServers:      []string{"stun.l.google.com:19302"},
		TrustedPeerCerts: []string{},
		Networks:         []Network{},
		Core: CoreConfig{
			BufferPackets:   256,
			MaxFrameBytes:   65535,
			ShutdownTimeout: 3 * time.Second,
		},
		Api: ApiConfig{
			Auth:        "bearer",
			BearerToken: "",
			RateLimit:   ApiRateLimit{RPS: 10, Burst: 20},
			Validation:  true,
		},
		Diag: DiagConfig{
			MtuProbeMax: 1500,
		},
		Updater: UpdaterConfig{
			Enabled:          false,
			Repo:             "",
			RequireSignature: false,
			PublicKey:        "",
		},
		Logging: LoggingConfig{
			Format: "json",
			Level:  "info",
		},
		Metrics: MetricsConfig{
			Enabled: false,
			Addr:    "127.0.0.1:9090",
		},
	}
}

// GenerateBearerToken returns a random 32-hex-character token.
func GenerateBearerToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
	// If bearer token is empty, auto-generate one for local management convenience.
	if strings.EqualFold(strings.TrimSpace(cfg.Api.Auth), "bearer") && strings.TrimSpace(cfg.Api.BearerToken) == "" {
		cfg.Api.BearerToken = GenerateBearerToken()
		_ = Save(cfg)
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
	// Core defaults if zero/unset
	if cfg.Core.BufferPackets <= 0 {
		cfg.Core.BufferPackets = 256
	}
	if cfg.Core.MaxFrameBytes <= 0 {
		cfg.Core.MaxFrameBytes = 65535
	}
	if cfg.Core.ShutdownTimeout <= 0 {
		cfg.Core.ShutdownTimeout = 3 * time.Second
	}
	// API defaults if zero/unset
	if strings.TrimSpace(cfg.Api.Auth) == "" {
		cfg.Api.Auth = "bearer"
	}
	if cfg.Api.RateLimit.RPS <= 0 {
		cfg.Api.RateLimit.RPS = 10
	}
	if cfg.Api.RateLimit.Burst <= 0 {
		cfg.Api.RateLimit.Burst = 20
	}
	// If Validation bool is left as default (false), set to true explicitly
	// unless user specified false in config file. We can't distinguish absent vs false with yaml easily,
	// so we assume false means user choice; only set true if both RPS and Burst were zero indicating defaults.
	// To keep behavior predictable, ensure Validation is true when Auth is bearer and BearerToken set by default.
	if cfg.Api.Validation == false && strings.TrimSpace(cfg.Api.BearerToken) == "" {
		cfg.Api.Validation = true
	}
	// Diag defaults
	if cfg.Diag.MtuProbeMax <= 0 {
		cfg.Diag.MtuProbeMax = 1500
	}
	// Logging defaults
	lf := strings.ToLower(strings.TrimSpace(cfg.Logging.Format))
	if lf == "" {
		cfg.Logging.Format = "json"
	} else if lf != "json" && lf != "text" {
		cfg.Logging.Format = "json"
	}
	lvl := strings.ToLower(strings.TrimSpace(cfg.Logging.Level))
	if lvl == "" {
		// Back-compat: use top-level LogLevel if set
		lv2 := strings.ToLower(strings.TrimSpace(cfg.LogLevel))
		if lv2 != "" {
			cfg.Logging.Level = lv2
		} else {
			cfg.Logging.Level = "info"
		}
	}
	// Metrics defaults
	if strings.TrimSpace(cfg.Metrics.Addr) == "" {
		cfg.Metrics.Addr = "127.0.0.1:9090"
	}
	// Updater defaults - keep conservative
	// Enabled: default false unless explicitly enabled in config.
	// Repo: leave empty unless user specifies (prevents accidental network calls).
	// RequireSignature/PublicKey remain as provided.
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
