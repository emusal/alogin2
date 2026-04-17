package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// JobStatus represents the lifecycle state of a background job.
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobDone      JobStatus = "done"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// BGJob represents one row in the bg_jobs table.
type BGJob struct {
	ID         string
	SessionID  string
	ServerHost string
	Command    string
	Status     JobStatus
	ExitCode   *int
	Output     string
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// JobRepo is the data-access interface for bg_jobs.
type JobRepo interface {
	Create(ctx context.Context, job *BGJob) error
	// ResolveID resolves a full UUID or a unique prefix to a full job ID.
	// Returns an error if the prefix is ambiguous or not found.
	ResolveID(ctx context.Context, prefix string) (string, error)
	Get(ctx context.Context, id string) (*BGJob, error)
	List(ctx context.Context, sessionID string) ([]*BGJob, error)
	SetRunning(ctx context.Context, id string, startedAt time.Time) error
	SetFinished(ctx context.Context, id string, status JobStatus, exitCode int, finishedAt time.Time) error
	AppendOutput(ctx context.Context, id, chunk string) error
	Cancel(ctx context.Context, id string) error
	// Delete removes a single job record regardless of status.
	Delete(ctx context.Context, id string) error
	// Purge deletes finished jobs (done/failed/cancelled). Pass all=true to delete every job.
	Purge(ctx context.Context, all bool) (int64, error)
}

type jobRepo struct{ db *sql.DB }

func (r *jobRepo) Create(ctx context.Context, job *BGJob) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO bg_jobs (id, session_id, server_host, command, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		job.ID, job.SessionID, job.ServerHost, job.Command,
		string(JobPending), job.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("job create: %w", err)
	}
	return nil
}

func (r *jobRepo) ResolveID(ctx context.Context, prefix string) (string, error) {
	if len(prefix) == 36 {
		return prefix, nil // already a full UUID
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM bg_jobs WHERE id LIKE ?`, prefix+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		ids = append(ids, id)
	}
	switch len(ids) {
	case 0:
		return "", fmt.Errorf("job %q not found", prefix)
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("prefix %q is ambiguous (%d matches)", prefix, len(ids))
	}
}

func (r *jobRepo) Get(ctx context.Context, id string) (*BGJob, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, session_id, server_host, command, status, exit_code, output,
		       created_at, started_at, finished_at
		FROM bg_jobs WHERE id = ?`, id)
	j, err := scanJob(row)
	if err != nil && err.Error() == "scan job: sql: no rows in result set" {
		return nil, nil
	}
	return j, err
}

func (r *jobRepo) List(ctx context.Context, sessionID string) ([]*BGJob, error) {
	q := `SELECT id, session_id, server_host, command, status, exit_code, output,
	             created_at, started_at, finished_at FROM bg_jobs`
	var args []any
	if sessionID != "" {
		q += " WHERE session_id = ?"
		args = append(args, sessionID)
	}
	q += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("job list: %w", err)
	}
	defer rows.Close()
	var jobs []*BGJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (r *jobRepo) SetRunning(ctx context.Context, id string, startedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE bg_jobs SET status='running', started_at=? WHERE id=?`,
		startedAt.UTC().Format(time.RFC3339), id)
	return err
}

func (r *jobRepo) SetFinished(ctx context.Context, id string, status JobStatus, exitCode int, finishedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE bg_jobs SET status=?, exit_code=?, finished_at=? WHERE id=?`,
		string(status), exitCode, finishedAt.UTC().Format(time.RFC3339), id)
	return err
}

func (r *jobRepo) AppendOutput(ctx context.Context, id, chunk string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE bg_jobs SET output = output || ? WHERE id = ?`, chunk, id)
	return err
}

func (r *jobRepo) Cancel(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE bg_jobs SET status='cancelled' WHERE id=? AND status IN ('pending','running')`, id)
	if err != nil {
		return fmt.Errorf("job cancel: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job %s is not cancellable (already finished or not found)", id)
	}
	return nil
}

func (r *jobRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM bg_jobs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("job delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job %s not found", id)
	}
	return nil
}

func (r *jobRepo) Purge(ctx context.Context, all bool) (int64, error) {
	var q string
	if all {
		q = `DELETE FROM bg_jobs`
	} else {
		q = `DELETE FROM bg_jobs WHERE status IN ('done','failed','cancelled')`
	}
	res, err := r.db.ExecContext(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("job purge: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanJob(s scanner) (*BGJob, error) {
	var j BGJob
	var exitCode sql.NullInt64
	var createdAt, startedAt, finishedAt sql.NullString
	err := s.Scan(
		&j.ID, &j.SessionID, &j.ServerHost, &j.Command, &j.Status,
		&exitCode, &j.Output,
		&createdAt, &startedAt, &finishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan job: %w", err)
	}
	if exitCode.Valid {
		v := int(exitCode.Int64)
		j.ExitCode = &v
	}
	if createdAt.Valid {
		t, _ := time.Parse(time.RFC3339, createdAt.String)
		j.CreatedAt = t
	}
	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339, startedAt.String)
		j.StartedAt = &t
	}
	if finishedAt.Valid {
		t, _ := time.Parse(time.RFC3339, finishedAt.String)
		j.FinishedAt = &t
	}
	return &j, nil
}
