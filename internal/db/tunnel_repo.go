package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/emusal/alogin2/internal/model"
)

// TunnelRepo defines CRUD operations for saved tunnel configurations.
type TunnelRepo interface {
	Create(ctx context.Context, t *model.Tunnel) error
	GetByID(ctx context.Context, id int64) (*model.Tunnel, error)
	GetByName(ctx context.Context, name string) (*model.Tunnel, error)
	ListAll(ctx context.Context) ([]*model.Tunnel, error)
	Update(ctx context.Context, t *model.Tunnel) error
	Delete(ctx context.Context, id int64) error
}

type tunnelRepo struct{ db *sql.DB }

func (r *tunnelRepo) Create(ctx context.Context, t *model.Tunnel) error {
	autoGW := 0
	if t.AutoGW {
		autoGW = 1
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO tunnels (name, server_id, direction, local_host, local_port, remote_host, remote_port, auto_gw)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Name, t.ServerID, string(t.Direction), t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort, autoGW,
	)
	if err != nil {
		return fmt.Errorf("create tunnel: %w", err)
	}
	id, _ := res.LastInsertId()
	t.ID = id
	return nil
}

func (r *tunnelRepo) GetByID(ctx context.Context, id int64) (*model.Tunnel, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, server_id, direction, local_host, local_port, remote_host, remote_port, auto_gw, created_at, updated_at
		 FROM tunnels WHERE id = ?`, id)
	return scanTunnel(row)
}

func (r *tunnelRepo) GetByName(ctx context.Context, name string) (*model.Tunnel, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, server_id, direction, local_host, local_port, remote_host, remote_port, auto_gw, created_at, updated_at
		 FROM tunnels WHERE name = ?`, name)
	return scanTunnel(row)
}

func (r *tunnelRepo) ListAll(ctx context.Context) ([]*model.Tunnel, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, server_id, direction, local_host, local_port, remote_host, remote_port, auto_gw, created_at, updated_at
		 FROM tunnels ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tunnels []*model.Tunnel
	for rows.Next() {
		t, err := scanTunnelRow(rows)
		if err != nil {
			return nil, err
		}
		tunnels = append(tunnels, t)
	}
	return tunnels, rows.Err()
}

func (r *tunnelRepo) Update(ctx context.Context, t *model.Tunnel) error {
	autoGW := 0
	if t.AutoGW {
		autoGW = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE tunnels SET name=?, server_id=?, direction=?, local_host=?, local_port=?, remote_host=?, remote_port=?, auto_gw=?, updated_at=?
		 WHERE id=?`,
		t.Name, t.ServerID, string(t.Direction), t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort, autoGW,
		time.Now().UTC().Format(time.RFC3339), t.ID,
	)
	return err
}

func (r *tunnelRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tunnels WHERE id = ?`, id)
	return err
}

// --- helpers ---

func scanTunnel(row *sql.Row) (*model.Tunnel, error) {
	t := &model.Tunnel{}
	var dir string
	var autoGW int
	var createdAt, updatedAt string
	err := row.Scan(&t.ID, &t.Name, &t.ServerID, &dir, &t.LocalHost, &t.LocalPort, &t.RemoteHost, &t.RemotePort, &autoGW, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan tunnel: %w", err)
	}
	t.Direction = model.TunnelDirection(dir)
	t.AutoGW = autoGW != 0
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return t, nil
}

func scanTunnelRow(rows *sql.Rows) (*model.Tunnel, error) {
	t := &model.Tunnel{}
	var dir string
	var autoGW int
	var createdAt, updatedAt string
	if err := rows.Scan(&t.ID, &t.Name, &t.ServerID, &dir, &t.LocalHost, &t.LocalPort, &t.RemoteHost, &t.RemotePort, &autoGW, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	t.Direction = model.TunnelDirection(dir)
	t.AutoGW = autoGW != 0
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return t, nil
}
