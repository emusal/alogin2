package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/emusal/alogin2/internal/model"
)

// AppServerRepo defines CRUD operations for app-server bindings.
type AppServerRepo interface {
	Create(ctx context.Context, as *model.AppServer) error
	GetByID(ctx context.Context, id int64) (*model.AppServer, error)
	GetByName(ctx context.Context, name string) (*model.AppServer, error)
	ListAll(ctx context.Context) ([]*model.AppServer, error)
	Update(ctx context.Context, as *model.AppServer) error
	Delete(ctx context.Context, id int64) error
}

type appServerRepo struct{ db *sql.DB }

func (r *appServerRepo) Create(ctx context.Context, as *model.AppServer) error {
	autoGW := 0
	if as.AutoGW {
		autoGW = 1
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO app_servers (name, server_id, plugin_name, auto_gw, description)
		 VALUES (?, ?, ?, ?, ?)`,
		as.Name, as.ServerID, as.PluginName, autoGW, as.Description,
	)
	if err != nil {
		return fmt.Errorf("create app_server: %w", err)
	}
	id, _ := res.LastInsertId()
	as.ID = id
	return nil
}

func (r *appServerRepo) GetByID(ctx context.Context, id int64) (*model.AppServer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, server_id, plugin_name, auto_gw, description, created_at, updated_at
		 FROM app_servers WHERE id = ?`, id)
	return scanAppServer(row)
}

func (r *appServerRepo) GetByName(ctx context.Context, name string) (*model.AppServer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, server_id, plugin_name, auto_gw, description, created_at, updated_at
		 FROM app_servers WHERE name = ?`, name)
	return scanAppServer(row)
}

func (r *appServerRepo) ListAll(ctx context.Context) ([]*model.AppServer, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, server_id, plugin_name, auto_gw, description, created_at, updated_at
		 FROM app_servers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.AppServer
	for rows.Next() {
		as, err := scanAppServerRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, as)
	}
	return list, rows.Err()
}

func (r *appServerRepo) Update(ctx context.Context, as *model.AppServer) error {
	autoGW := 0
	if as.AutoGW {
		autoGW = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_servers SET name=?, server_id=?, plugin_name=?, auto_gw=?, description=?, updated_at=?
		 WHERE id=?`,
		as.Name, as.ServerID, as.PluginName, autoGW, as.Description,
		time.Now().UTC().Format(time.RFC3339), as.ID,
	)
	return err
}

func (r *appServerRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM app_servers WHERE id = ?`, id)
	return err
}

// --- helpers ---

func scanAppServer(row *sql.Row) (*model.AppServer, error) {
	as := &model.AppServer{}
	var autoGW int
	var createdAt, updatedAt string
	err := row.Scan(&as.ID, &as.Name, &as.ServerID, &as.PluginName, &autoGW, &as.Description, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan app_server: %w", err)
	}
	as.AutoGW = autoGW != 0
	as.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	as.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return as, nil
}

func scanAppServerRow(rows *sql.Rows) (*model.AppServer, error) {
	as := &model.AppServer{}
	var autoGW int
	var createdAt, updatedAt string
	if err := rows.Scan(&as.ID, &as.Name, &as.ServerID, &as.PluginName, &autoGW, &as.Description, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	as.AutoGW = autoGW != 0
	as.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	as.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return as, nil
}
