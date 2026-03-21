package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/emusal/alogin2/internal/model"
)

// AliasRepo defines CRUD operations for host aliases.
type AliasRepo interface {
	Create(ctx context.Context, a *model.Alias) error
	GetByName(ctx context.Context, name string) (*model.Alias, error)
	ListAll(ctx context.Context) ([]*model.Alias, error)
	Delete(ctx context.Context, id int64) error
}

type aliasRepo struct{ db *sql.DB }

func (r *aliasRepo) Create(ctx context.Context, a *model.Alias) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO aliases (alias_name, server_id, user) VALUES (?, ?, ?)`,
		a.Name, a.ServerID, a.User)
	if err != nil {
		return fmt.Errorf("create alias: %w", err)
	}
	return nil
}

func (r *aliasRepo) GetByName(ctx context.Context, name string) (*model.Alias, error) {
	a := &model.Alias{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, alias_name, server_id, user FROM aliases WHERE alias_name = ?`, name).
		Scan(&a.ID, &a.Name, &a.ServerID, &a.User)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (r *aliasRepo) ListAll(ctx context.Context) ([]*model.Alias, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, alias_name, server_id, user FROM aliases ORDER BY alias_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aliases []*model.Alias
	for rows.Next() {
		a := &model.Alias{}
		if err := rows.Scan(&a.ID, &a.Name, &a.ServerID, &a.User); err != nil {
			return nil, err
		}
		aliases = append(aliases, a)
	}
	return aliases, rows.Err()
}

func (r *aliasRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM aliases WHERE id = ?`, id)
	return err
}
