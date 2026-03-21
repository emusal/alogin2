package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/emusal/alogin2/internal/model"
)

// HostRepo defines CRUD operations for the local hosts table.
type HostRepo interface {
	Create(ctx context.Context, h *model.LocalHost) error
	GetByHostname(ctx context.Context, hostname string) (*model.LocalHost, error)
	ListAll(ctx context.Context) ([]*model.LocalHost, error)
	Update(ctx context.Context, h *model.LocalHost) error
	Delete(ctx context.Context, id int64) error
	// Resolve returns the IP for hostname if present, otherwise returns hostname unchanged.
	Resolve(ctx context.Context, hostname string) string
}

type hostRepo struct{ db *sql.DB }

func (r *hostRepo) Create(ctx context.Context, h *model.LocalHost) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO local_hosts (hostname, ip, description) VALUES (?, ?, ?)`,
		h.Hostname, h.IP, h.Description,
	)
	if err != nil {
		return fmt.Errorf("create local host: %w", err)
	}
	id, _ := res.LastInsertId()
	h.ID = id
	return nil
}

func (r *hostRepo) GetByHostname(ctx context.Context, hostname string) (*model.LocalHost, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, hostname, ip, description, created_at, updated_at FROM local_hosts WHERE hostname = ?`,
		hostname)
	return scanLocalHost(row)
}

func (r *hostRepo) ListAll(ctx context.Context) ([]*model.LocalHost, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, hostname, ip, description, created_at, updated_at FROM local_hosts ORDER BY hostname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []*model.LocalHost
	for rows.Next() {
		h, err := scanLocalHostRow(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (r *hostRepo) Update(ctx context.Context, h *model.LocalHost) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE local_hosts SET ip=?, description=?, updated_at=? WHERE id=?`,
		h.IP, h.Description, time.Now().UTC().Format(time.RFC3339), h.ID,
	)
	return err
}

func (r *hostRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM local_hosts WHERE id = ?`, id)
	return err
}

func (r *hostRepo) Resolve(ctx context.Context, hostname string) string {
	h, err := r.GetByHostname(ctx, hostname)
	if err != nil || h == nil {
		return hostname
	}
	return h.IP
}

// --- helpers ---

func scanLocalHost(row *sql.Row) (*model.LocalHost, error) {
	h := &model.LocalHost{}
	var createdAt, updatedAt string
	err := row.Scan(&h.ID, &h.Hostname, &h.IP, &h.Description, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan local host: %w", err)
	}
	h.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	h.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return h, nil
}

func scanLocalHostRow(rows *sql.Rows) (*model.LocalHost, error) {
	h := &model.LocalHost{}
	var createdAt, updatedAt string
	if err := rows.Scan(&h.ID, &h.Hostname, &h.IP, &h.Description, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	h.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	h.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return h, nil
}
