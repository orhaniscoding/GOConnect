package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goconnect/internal/config"
)

// Result represents an update check result.
type Result struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	URL       string `json:"url,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Check queries the configured GitHub repo for the latest release. It loads config from disk.
func Check() (*Result, error) {
	cfg, _ := config.Load() // best-effort
	return CheckWithConfig(cfg, &http.Client{Timeout: 10 * time.Second})
}

// CheckWithConfig is the testable variant.
func CheckWithConfig(cfg *config.Config, client httpDoer) (*Result, error) {
	if cfg == nil || !cfg.Updater.Enabled {
		return &Result{Available: false, Version: "", Notes: "updater disabled"}, nil
	}
	repo := strings.TrimSpace(cfg.Updater.Repo)
	if repo == "" || !strings.Contains(repo, "/") {
		return &Result{Available: false, Version: "", Notes: "updater repo not configured"}, nil
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, _ := http.NewRequest("GET", url, nil)
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("github api: %s: %s", resp.Status, string(b))
	}
	var rel struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	// Choose an .exe asset for Windows
	assetURL := ""
	for _, a := range rel.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
			assetURL = a.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		return &Result{Available: false, Version: rel.TagName, Notes: "no windows asset found"}, nil
	}
	// We cannot reliably compare local version without build info; surface version and URL.
	return &Result{Available: true, Version: rel.TagName, URL: assetURL}, nil
}

// Apply performs an update using config from disk, staging a new binary alongside the current executable.
func Apply() error {
	cfg, _ := config.Load()
	return ApplyWithConfig(cfg, &http.Client{Timeout: 30 * time.Second})
}

// ApplyWithConfig downloads and stages the update. On Windows, replacing the running binary is not possible;
// instead, this writes a .new sibling file and returns, expecting the service manager or a subsequent run to finalize.
func ApplyWithConfig(cfg *config.Config, client httpDoer) error {
	if cfg == nil || !cfg.Updater.Enabled {
		return errors.New("updater disabled")
	}
	res, err := CheckWithConfig(cfg, client)
	if err != nil {
		return err
	}
	if !res.Available || res.URL == "" {
		return errors.New("no update available")
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable path: %w", err)
	}
	dir := filepath.Dir(exe)
	staged := exe + ".new"
	f, err := os.Create(staged)
	if err != nil {
		return fmt.Errorf("create staged: %w", err)
	}
	defer f.Close()
	// Download
	req, _ := http.NewRequest("GET", res.URL, nil)
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("download: %s: %s", resp.Status, string(b))
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	_ = sum // reserved: optionally compare with .sha256 asset in the future

	// Best-effort to make executable
	_ = f.Chmod(0o755)

	// Attempt atomic swap if the binary is not locked. Otherwise, leave .new for next restart.
	backup := exe + ".bak"
	if err := os.Rename(exe, backup); err != nil {
		// Likely locked; keep staged for next run
		// Write a marker file to indicate staging
		_ = os.WriteFile(filepath.Join(dir, ".update-staged"), []byte(time.Now().Format(time.RFC3339)), 0o644)
		return nil
	}
	// Now move staged to exe path
	if err := os.Rename(staged, exe); err != nil {
		// Try to roll back
		_ = os.Rename(backup, exe)
		return fmt.Errorf("activate update: %w", err)
	}
	// Keep backup for manual rollback
	return nil
}

// ApplyToPath is a helper for tests to apply an update to a specific target path (not the running executable).
func ApplyToPath(cfg *config.Config, client httpDoer, targetPath, downloadURL string) error {
	if cfg == nil || !cfg.Updater.Enabled {
		return errors.New("updater disabled")
	}
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	staged := targetPath + ".new"
	f, err := os.Create(staged)
	if err != nil {
		return err
	}
	defer f.Close()
	req, _ := http.NewRequest("GET", downloadURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("download: %s: %s", resp.Status, string(b))
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	_ = f.Chmod(0o755)
	backup := targetPath + ".bak"
	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(staged, targetPath); err != nil {
		// try rollback
		_ = os.Rename(backup, targetPath)
		return err
	}
	return nil
}
