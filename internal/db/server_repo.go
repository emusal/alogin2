package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/emusal/alogin2/internal/model"
)

// ServerRepo defines CRUD operations for servers.
type ServerRepo interface {
	Create(ctx context.Context, s *model.Server, password string) error
	GetByID(ctx context.Context, id int64) (*model.Server, error)
	GetByHost(ctx context.Context, host, user string) (*model.Server, error)
	ListAll(ctx context.Context) ([]*model.Server, error)
	Search(ctx context.Context, query string) ([]*model.Server, error)
	Update(ctx context.Context, s *model.Server, newPassword string) error
	Delete(ctx context.Context, id int64) error
}

type serverRepo struct{ db *sql.DB }

func (r *serverRepo) Create(ctx context.Context, s *model.Server, password string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO servers (protocol, host, user, password, port, gateway_id, gateway_server_id, locale, device_type, note, policy_yaml, system_prompt)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(s.Protocol), s.Host, s.User, password, s.Port,
		nullInt64(s.GatewayID), nullInt64(s.GatewayServerID), s.Locale,
		deviceTypeOrDefault(s.DeviceType), s.Note,
		nullableText(s.PolicyYAML), nullableText(s.SystemPrompt),
	)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return nil
}

func (r *serverRepo) GetByID(ctx context.Context, id int64) (*model.Server, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, protocol, host, user, password, port, gateway_id, gateway_server_id, locale, device_type, note, policy_yaml, system_prompt, created_at, updated_at
		 FROM servers WHERE id = ?`, id)
	return scanServer(row)
}

func (r *serverRepo) GetByHost(ctx context.Context, host, user string) (*model.Server, error) {
	var row *sql.Row
	if user == "" {
		row = r.db.QueryRowContext(ctx,
			`SELECT id, protocol, host, user, password, port, gateway_id, gateway_server_id, locale, device_type, note, policy_yaml, system_prompt, created_at, updated_at
			 FROM servers WHERE host = ? ORDER BY id LIMIT 1`, host)
	} else {
		row = r.db.QueryRowContext(ctx,
			`SELECT id, protocol, host, user, password, port, gateway_id, gateway_server_id, locale, device_type, note, policy_yaml, system_prompt, created_at, updated_at
			 FROM servers WHERE host = ? AND user = ?`, host, user)
	}
	return scanServer(row)
}

func (r *serverRepo) ListAll(ctx context.Context) ([]*model.Server, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, protocol, host, user, password, port, gateway_id, gateway_server_id, locale, device_type, note, policy_yaml, system_prompt, created_at, updated_at
		 FROM servers ORDER BY host, user`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServers(rows)
}

func (r *serverRepo) Search(ctx context.Context, query string) ([]*model.Server, error) {
	like := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, protocol, host, user, password, port, gateway_id, gateway_server_id, locale, device_type, note, policy_yaml, system_prompt, created_at, updated_at
		 FROM servers WHERE host LIKE ? OR user LIKE ? OR note LIKE ? ORDER BY host`,
		like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServers(rows)
}

func (r *serverRepo) Update(ctx context.Context, s *model.Server, newPassword string) error {
	args := []any{
		string(s.Protocol), s.User, s.Port, nullInt64(s.GatewayID), nullInt64(s.GatewayServerID), s.Locale,
		deviceTypeOrDefault(s.DeviceType), s.Note,
		nullableText(s.PolicyYAML), nullableText(s.SystemPrompt),
		time.Now().UTC().Format(time.RFC3339), s.ID,
	}
	query := `UPDATE servers SET protocol=?, user=?, port=?, gateway_id=?, gateway_server_id=?, locale=?, device_type=?, note=?, policy_yaml=?, system_prompt=?, updated_at=? WHERE id=?`
	if newPassword != "" {
		query = `UPDATE servers SET protocol=?, user=?, port=?, gateway_id=?, gateway_server_id=?, locale=?, device_type=?, note=?, policy_yaml=?, system_prompt=?, updated_at=?, password=? WHERE id=?`
		args = append(args[:11], newPassword, s.ID)
	}
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *serverRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	return err
}

// PasswordFor returns the raw password column value (may be "_HIDDEN_").
func PasswordFor(ctx context.Context, db *sql.DB, serverID int64) (string, error) {
	var pwd string
	err := db.QueryRowContext(ctx, `SELECT password FROM servers WHERE id = ?`, serverID).Scan(&pwd)
	return pwd, err
}

// --- helpers ---

func scanServer(row *sql.Row) (*model.Server, error) {
	s := &model.Server{}
	var gwID, gwSrvID sql.NullInt64
	var policyYAML, systemPrompt sql.NullString
	var createdAt, updatedAt, deviceType string
	err := row.Scan(&s.ID, &s.Protocol, &s.Host, &s.User, new(string), &s.Port,
		&gwID, &gwSrvID, &s.Locale, &deviceType, &s.Note,
		&policyYAML, &systemPrompt,
		&createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan server: %w", err)
	}
	if gwID.Valid {
		v := gwID.Int64
		s.GatewayID = &v
	}
	if gwSrvID.Valid {
		v := gwSrvID.Int64
		s.GatewayServerID = &v
	}
	s.DeviceType = model.DeviceType(deviceType)
	s.PolicyYAML = policyYAML.String
	s.SystemPrompt = systemPrompt.String
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return s, nil
}

func scanServers(rows *sql.Rows) ([]*model.Server, error) {
	var servers []*model.Server
	for rows.Next() {
		s := &model.Server{}
		var gwID, gwSrvID sql.NullInt64
		var policyYAML, systemPrompt sql.NullString
		var createdAt, updatedAt, deviceType string
		if err := rows.Scan(&s.ID, &s.Protocol, &s.Host, &s.User, new(string), &s.Port,
			&gwID, &gwSrvID, &s.Locale, &deviceType, &s.Note,
			&policyYAML, &systemPrompt,
			&createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if gwID.Valid {
			v := gwID.Int64
			s.GatewayID = &v
		}
		if gwSrvID.Valid {
			v := gwSrvID.Int64
			s.GatewayServerID = &v
		}
		s.DeviceType = model.DeviceType(deviceType)
		s.PolicyYAML = policyYAML.String
		s.SystemPrompt = systemPrompt.String
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		servers = append(servers, s)
	}
	return servers, rows.Err()
}

func nullInt64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// nullableText returns sql.NullString with Valid=false when s is empty,
// storing NULL in the database (sentinel meaning "use global default").
func nullableText(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func deviceTypeOrDefault(dt model.DeviceType) string {
	if dt == "" {
		return string(model.DeviceLinux)
	}
	return string(dt)
}
