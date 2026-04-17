package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/emusal/alogin2/internal/model"
)

// AgentMemoryRepo defines operations for agent memory notes.
type AgentMemoryRepo interface {
	Add(ctx context.Context, serverID int64, content string) (*model.AgentMemory, error)
	ListByServer(ctx context.Context, serverID int64) ([]*model.AgentMemory, error)
	Delete(ctx context.Context, id int64) error
}

type agentMemoryRepo struct{ db *sql.DB }

func (r *agentMemoryRepo) Add(ctx context.Context, serverID int64, content string) (*model.AgentMemory, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO agent_memory (server_id, content) VALUES (?, ?)`, serverID, content)
	if err != nil {
		return nil, fmt.Errorf("add agent memory: %w", err)
	}
	id, _ := res.LastInsertId()
	row := r.db.QueryRowContext(ctx,
		`SELECT id, server_id, content, created_at FROM agent_memory WHERE id = ?`, id)
	return scanAgentMemory(row)
}

func (r *agentMemoryRepo) ListByServer(ctx context.Context, serverID int64) ([]*model.AgentMemory, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, server_id, content, created_at FROM agent_memory WHERE server_id = ? ORDER BY created_at`,
		serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.AgentMemory
	for rows.Next() {
		m, err := scanAgentMemoryRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *agentMemoryRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM agent_memory WHERE id = ?`, id)
	return err
}

func scanAgentMemory(row *sql.Row) (*model.AgentMemory, error) {
	m := &model.AgentMemory{}
	var createdAt string
	if err := row.Scan(&m.ID, &m.ServerID, &m.Content, &createdAt); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("scan agent_memory: %w", err)
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return m, nil
}

func scanAgentMemoryRow(rows *sql.Rows) (*model.AgentMemory, error) {
	m := &model.AgentMemory{}
	var createdAt string
	if err := rows.Scan(&m.ID, &m.ServerID, &m.Content, &createdAt); err != nil {
		return nil, err
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return m, nil
}
