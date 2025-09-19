import (
	"controller/models"
// --- ControllerStore interface implementation for SQLite ---
func (s *SQLiteStore) GetNetworkSettings(networkID string) (*models.NetworkSettings, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	row := s.db.QueryRow(`SELECT data FROM settings WHERE id = ?`, networkID)
	var data string
	if err := row.Scan(&data); err != nil {
		return nil, false
	}
	var ns models.NetworkSettings
	if err := json.Unmarshal([]byte(data), &ns); err != nil {
		return nil, false
	}
	return &ns, true
}

func (s *SQLiteStore) SetNetworkSettings(networkID string, ns *models.NetworkSettings) {
	s.lock.Lock()
	defer s.lock.Unlock()
	b, _ := json.Marshal(ns)
	_, _ = s.db.Exec(`INSERT OR REPLACE INTO settings (id, data, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, networkID, string(b))
}

func (s *SQLiteStore) GetMembershipPreferences(networkID, nodeID string) (*models.MembershipPreferences, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	key := networkID + ":" + nodeID
	row := s.db.QueryRow(`SELECT data FROM settings WHERE id = ?`, key)
	var data string
	if err := row.Scan(&data); err != nil {
		return nil, false
	}
	var mp models.MembershipPreferences
	if err := json.Unmarshal([]byte(data), &mp); err != nil {
		return nil, false
	}
	return &mp, true
}

func (s *SQLiteStore) SetMembershipPreferences(networkID, nodeID string, mp *models.MembershipPreferences) {
	s.lock.Lock()
	defer s.lock.Unlock()
	key := networkID + ":" + nodeID
	b, _ := json.Marshal(mp)
	_, _ = s.db.Exec(`INSERT OR REPLACE INTO settings (id, data, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, key, string(b))
}
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db   *sql.DB
	lock sync.RWMutex
	path string
}

func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "controller.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteStore{db: db, path: dbPath}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) initSchema() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS networks (
			id TEXT PRIMARY KEY,
			name TEXT,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS members (
			id TEXT PRIMARY KEY,
			network_id TEXT,
			name TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			id TEXT PRIMARY KEY,
			data TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS tokens (
			id TEXT PRIMARY KEY,
			network_id TEXT,
			type TEXT,
			value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS policies (
			id TEXT PRIMARY KEY,
			network_id TEXT,
			data TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_members_network_id ON members(network_id);`,
		`CREATE INDEX IF NOT EXISTS idx_tokens_network_id ON tokens(network_id);`,
		`CREATE INDEX IF NOT EXISTS idx_policies_network_id ON policies(network_id);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.db.Close()
}

// Example CRUD for networks
type Network struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
}

func (s *SQLiteStore) CreateNetwork(ctx context.Context, n *Network) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO networks (id, name, description) VALUES (?, ?, ?)`, n.ID, n.Name, n.Description)
	return err
}

func (s *SQLiteStore) GetNetwork(ctx context.Context, id string) (*Network, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	row := s.db.QueryRowContext(ctx, `SELECT id, name, description, created_at FROM networks WHERE id = ?`, id)
	var n Network
	var created string
	if err := row.Scan(&n.ID, &n.Name, &n.Description, &created); err != nil {
		return nil, err
	}
	t, _ := time.Parse(time.RFC3339, created)
	n.CreatedAt = t
	return &n, nil
}

func (s *SQLiteStore) UpdateNetwork(ctx context.Context, n *Network) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE networks SET name = ?, description = ? WHERE id = ?`, n.Name, n.Description, n.ID)
	return err
}

func (s *SQLiteStore) DeleteNetwork(ctx context.Context, id string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM networks WHERE id = ?`, id)
	return err
}

// Settings CRUD (JSON blob)
type Settings struct {
	ID   string
	Data map[string]interface{}
}

func (s *SQLiteStore) SaveSettings(ctx context.Context, st *Settings) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	b, err := json.Marshal(st.Data)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR REPLACE INTO settings (id, data, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, st.ID, string(b))
	return err
}

func (s *SQLiteStore) LoadSettings(ctx context.Context, id string) (*Settings, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	row := s.db.QueryRowContext(ctx, `SELECT data FROM settings WHERE id = ?`, id)
	var data string
	if err := row.Scan(&data); err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return nil, err
	}
	return &Settings{ID: id, Data: m}, nil
}

// Backup: copies the DB file to a backup path
func (s *SQLiteStore) Backup(backupPath string) error {
	s.lock.RLock()
	defer s.lock.RUnlock()
	in, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(backupPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
