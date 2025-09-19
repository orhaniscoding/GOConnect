package store

import (
	"database/sql"
	"fmt"
)

// Simple migration helper: runs a list of SQL statements in a transaction.
func Migrate(db *sql.DB, stmts []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return tx.Commit()
}
