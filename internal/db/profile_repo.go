package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/emusal/alogin2/internal/model"
)

// ProfileRepo defines CRUD and activation operations for network profiles.
type ProfileRepo interface {
	Create(ctx context.Context, name, description string, routeID *int64) (*model.Profile, error)
	GetByID(ctx context.Context, id int64) (*model.Profile, error)
	GetByName(ctx context.Context, name string) (*model.Profile, error)
	ListAll(ctx context.Context) ([]*model.Profile, error)
	Update(ctx context.Context, p *model.Profile, routeID *int64) error
	Delete(ctx context.Context, id int64) error

	// GetActive returns the currently active profile, or nil if none is set.
	GetActive(ctx context.Context) (*model.Profile, error)
	// SetActive atomically deactivates all profiles and activates the given one.
	// Pass id=0 to deactivate all ("profile use none").
	SetActive(ctx context.Context, id int64) error
}

type profileRepo struct{ db *sql.DB }

func (r *profileRepo) Create(ctx context.Context, name, description string, routeID *int64) (*model.Profile, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := tx.ExecContext(ctx,
		`INSERT INTO profiles(name, description, is_active, created_at, updated_at)
		 VALUES (?, ?, 0, ?, ?)`,
		name, description, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create profile: %w", err)
	}
	id, _ := res.LastInsertId()

	if routeID != nil {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO profile_gateways(profile_id, route_id) VALUES (?, ?)`,
			id, *routeID,
		); err != nil {
			return nil, fmt.Errorf("link profile gateway: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, id)
}

func (r *profileRepo) GetByID(ctx context.Context, id int64) (*model.Profile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT p.id, p.name, p.description, p.is_active, pg.route_id, p.created_at, p.updated_at
		 FROM profiles p
		 LEFT JOIN profile_gateways pg ON pg.profile_id = p.id
		 WHERE p.id = ?`, id)
	return scanProfile(row)
}

func (r *profileRepo) GetByName(ctx context.Context, name string) (*model.Profile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT p.id, p.name, p.description, p.is_active, pg.route_id, p.created_at, p.updated_at
		 FROM profiles p
		 LEFT JOIN profile_gateways pg ON pg.profile_id = p.id
		 WHERE p.name = ?`, name)
	return scanProfile(row)
}

func (r *profileRepo) GetActive(ctx context.Context) (*model.Profile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT p.id, p.name, p.description, p.is_active, pg.route_id, p.created_at, p.updated_at
		 FROM profiles p
		 LEFT JOIN profile_gateways pg ON pg.profile_id = p.id
		 WHERE p.is_active = 1
		 LIMIT 1`)
	p, err := scanProfile(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *profileRepo) ListAll(ctx context.Context) ([]*model.Profile, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.name, p.description, p.is_active, pg.route_id, p.created_at, p.updated_at
		 FROM profiles p
		 LEFT JOIN profile_gateways pg ON pg.profile_id = p.id
		 ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*model.Profile
	for rows.Next() {
		p, err := scanProfileRow(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (r *profileRepo) Update(ctx context.Context, p *model.Profile, routeID *int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`UPDATE profiles SET name=?, description=?, updated_at=? WHERE id=?`,
		p.Name, p.Description, now, p.ID,
	); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}

	// Upsert gateway association.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM profile_gateways WHERE profile_id=?`, p.ID,
	); err != nil {
		return fmt.Errorf("clear profile gateway: %w", err)
	}
	if routeID != nil {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO profile_gateways(profile_id, route_id) VALUES (?, ?)`,
			p.ID, *routeID,
		); err != nil {
			return fmt.Errorf("set profile gateway: %w", err)
		}
	}

	return tx.Commit()
}

func (r *profileRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM profiles WHERE id=?`, id)
	return err
}

func (r *profileRepo) SetActive(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now().UTC().Format(time.RFC3339)

	// Deactivate all.
	if _, err := tx.ExecContext(ctx,
		`UPDATE profiles SET is_active=0, updated_at=? WHERE is_active=1`, now,
	); err != nil {
		return fmt.Errorf("deactivate profiles: %w", err)
	}

	// Activate the chosen one (id=0 means "none" → just deactivate all).
	if id != 0 {
		res, err := tx.ExecContext(ctx,
			`UPDATE profiles SET is_active=1, updated_at=? WHERE id=?`, now, id,
		)
		if err != nil {
			return fmt.Errorf("activate profile: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("profile %d not found", id)
		}
	}

	return tx.Commit()
}

// scanProfile scans a single *sql.Row into a Profile.
func scanProfile(row *sql.Row) (*model.Profile, error) {
	var (
		p       model.Profile
		routeID sql.NullInt64
		createdAt, updatedAt string
	)
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.IsActive, &routeID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if routeID.Valid {
		v := routeID.Int64
		p.GatewayRouteID = &v
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &p, nil
}

// scanProfileRow scans a *sql.Rows row into a Profile.
func scanProfileRow(rows *sql.Rows) (*model.Profile, error) {
	var (
		p       model.Profile
		routeID sql.NullInt64
		createdAt, updatedAt string
	)
	err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.IsActive, &routeID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if routeID.Valid {
		v := routeID.Int64
		p.GatewayRouteID = &v
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &p, nil
}
