package vault

import (
	"context"
	"database/sql"
	"fmt"
)

// PlaintextVault reads passwords directly from the servers.password column.
// This is the legacy / migration-compatibility backend.
type PlaintextVault struct {
	db *sql.DB
}

// NewPlaintext creates a vault backed by the DB password column.
func NewPlaintext(db *sql.DB) *PlaintextVault {
	return &PlaintextVault{db: db}
}

func (v *PlaintextVault) Name() string { return "plaintext" }

func (v *PlaintextVault) Get(account string) (string, error) {
	// account is "user@host"
	user, host := splitAccount(account)
	var pwd string
	err := v.db.QueryRowContext(context.Background(),
		`SELECT password FROM servers WHERE host = ? AND user = ? LIMIT 1`,
		host, user).Scan(&pwd)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if pwd == "_HIDDEN_" {
		return "", ErrNotFound
	}
	return pwd, nil
}

func (v *PlaintextVault) Set(account, password string) error {
	user, host := splitAccount(account)
	res, err := v.db.ExecContext(context.Background(),
		`UPDATE servers SET password = ? WHERE host = ? AND user = ?`,
		password, host, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("server %s not found", account)
	}
	return nil
}

func (v *PlaintextVault) Delete(account string) error {
	user, host := splitAccount(account)
	_, err := v.db.ExecContext(context.Background(),
		`UPDATE servers SET password = '_HIDDEN_' WHERE host = ? AND user = ?`,
		host, user)
	return err
}

func splitAccount(account string) (user, host string) {
	for i := len(account) - 1; i >= 0; i-- {
		if account[i] == '@' {
			return account[:i], account[i+1:]
		}
	}
	return "", account
}
