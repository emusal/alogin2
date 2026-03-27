package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AuditEntry represents one row in the audit_log table.
type AuditEntry struct {
	ID           int64
	Timestamp    string
	Event        string
	AgentID      string
	ServerID     *int64
	ServerHost   string
	ClusterID    *int64
	ClusterName  string
	Commands     []string
	Intent       string
	TimeoutSec   int
	PolicyAction string // "allow", "deny", "require_approval", or ""
	ApprovedBy   string // approval token if HITL-approved, else ""
	CreatedAt    time.Time
}

// AuditListOpts filters results from AuditRepo.List.
type AuditListOpts struct {
	AgentID   string
	ServerID  *int64
	EventType string
	Since     time.Time
	Limit     int // 0 = default 50
	Offset    int
}

// AuditRepo defines operations for the audit_log table.
type AuditRepo interface {
	Insert(ctx context.Context, e AuditEntry) (int64, error)
	List(ctx context.Context, opts AuditListOpts) ([]*AuditEntry, error)
	Count(ctx context.Context) (int64, error)
}

type auditRepo struct{ db *sql.DB }

func (r *auditRepo) Insert(ctx context.Context, e AuditEntry) (int64, error) {
	cmdsJSON, err := json.Marshal(e.Commands)
	if err != nil {
		cmdsJSON = []byte("[]")
	}
	ts := e.Timestamp
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_log
			(timestamp, event, agent_id, server_id, server_host, cluster_id, cluster_name,
			 commands, intent, timeout_sec, policy_action, approved_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ts,
		e.Event,
		e.AgentID,
		nullInt64Ptr(e.ServerID),
		e.ServerHost,
		nullInt64Ptr(e.ClusterID),
		e.ClusterName,
		string(cmdsJSON),
		e.Intent,
		e.TimeoutSec,
		nullString(e.PolicyAction),
		nullString(e.ApprovedBy),
	)
	if err != nil {
		return 0, fmt.Errorf("audit insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (r *auditRepo) List(ctx context.Context, opts AuditListOpts) ([]*AuditEntry, error) {
	where := []string{}
	args := []any{}

	if opts.AgentID != "" {
		where = append(where, "agent_id = ?")
		args = append(args, opts.AgentID)
	}
	if opts.ServerID != nil {
		where = append(where, "server_id = ?")
		args = append(args, *opts.ServerID)
	}
	if opts.EventType != "" {
		where = append(where, "event = ?")
		args = append(args, opts.EventType)
	}
	if !opts.Since.IsZero() {
		where = append(where, "created_at >= ?")
		args = append(args, opts.Since.UTC().Format(time.RFC3339))
	}

	q := `SELECT id, timestamp, event, agent_id, server_id, server_host, cluster_id, cluster_name,
	             commands, intent, timeout_sec, policy_action, approved_by, created_at
	      FROM audit_log`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY created_at DESC"

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	q += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, opts.Offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		e, err := scanAuditRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *auditRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&n)
	return n, err
}

// --- helpers ---

func scanAuditRow(rows *sql.Rows) (*AuditEntry, error) {
	e := &AuditEntry{}
	var serverID, clusterID sql.NullInt64
	var policyAction, approvedBy sql.NullString
	var cmdsJSON, createdAt string

	if err := rows.Scan(
		&e.ID, &e.Timestamp, &e.Event, &e.AgentID,
		&serverID, &e.ServerHost,
		&clusterID, &e.ClusterName,
		&cmdsJSON, &e.Intent, &e.TimeoutSec,
		&policyAction, &approvedBy,
		&createdAt,
	); err != nil {
		return nil, fmt.Errorf("scan audit row: %w", err)
	}

	if serverID.Valid {
		v := serverID.Int64
		e.ServerID = &v
	}
	if clusterID.Valid {
		v := clusterID.Int64
		e.ClusterID = &v
	}
	if policyAction.Valid {
		e.PolicyAction = policyAction.String
	}
	if approvedBy.Valid {
		e.ApprovedBy = approvedBy.String
	}
	_ = json.Unmarshal([]byte(cmdsJSON), &e.Commands)
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return e, nil
}

func nullInt64Ptr(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
