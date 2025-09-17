package api

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Files will live under ProgramData/GOConnect/state
func stateBaseDir() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\\ProgramData`
	}
	return filepath.Join(base, "GOConnect", "state")
}

func ensureStateDir() error {
	return os.MkdirAll(stateBaseDir(), 0o755)
}

const (
	fileNetworkSettings = "network_settings.json"
	fileMemberPrefs     = "member_prefs.json"
)

// persistedStructures groups data to persist (future extension: single file)
type persistedStructures struct {
	NetworkSettings   map[string]*NetworkSettingsState   `json:"network_settings"`
	MemberPreferences map[string]*MemberPreferencesState `json:"member_preferences"`
}

// saveAll writes both structures; currently best-effort, errors are returned but caller may choose to log only.
func (a *API) saveAll() error {
	if err := ensureStateDir(); err != nil {
		return err
	}
	a.netMu.RLock()
	ns := make(map[string]*NetworkSettingsState, len(a.networkSettings))
	for k, v := range a.networkSettings {
		cpy := *v
		ns[k] = &cpy
	}
	mp := make(map[string]*MemberPreferencesState, len(a.memberPreferences))
	for k, v := range a.memberPreferences {
		cpy := *v
		mp[k] = &cpy
	}
	a.netMu.RUnlock()

	data := persistedStructures{NetworkSettings: ns, MemberPreferences: mp}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(stateBaseDir(), "state.tmp")
	finalPath := filepath.Join(stateBaseDir(), "state.json")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, finalPath)
}

var loadOnce sync.Once
var loadErr error

// loadAll loads persisted state once; subsequent calls are no-ops.
func (a *API) loadAllOnce() error {
	loadOnce.Do(func() { loadErr = a.loadAllLocked() })
	return loadErr
}

func (a *API) loadAllLocked() error {
	path := filepath.Join(stateBaseDir(), "state.json")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	var ps persistedStructures
	if err := json.Unmarshal(b, &ps); err != nil {
		return err
	}
	a.netMu.Lock()
	if ps.NetworkSettings != nil {
		a.networkSettings = ps.NetworkSettings
	}
	if ps.MemberPreferences != nil {
		a.memberPreferences = ps.MemberPreferences
	}
	a.netMu.Unlock()
	return nil
}
