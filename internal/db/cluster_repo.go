package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/emusal/alogin2/internal/model"
)

// ClusterRepo defines CRUD operations for clusters.
type ClusterRepo interface {
	Create(ctx context.Context, name string, members []model.ClusterMember) (*model.Cluster, error)
	GetByID(ctx context.Context, id int64) (*model.Cluster, error)
	GetByName(ctx context.Context, name string) (*model.Cluster, error)
	ListAll(ctx context.Context) ([]*model.Cluster, error)
	Update(ctx context.Context, id int64, name string, members []model.ClusterMember) (*model.Cluster, error)
	Delete(ctx context.Context, id int64) error
}

type clusterRepo struct{ db *sql.DB }

func (r *clusterRepo) Create(ctx context.Context, name string, members []model.ClusterMember) (*model.Cluster, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `INSERT INTO clusters (name) VALUES (?)`, name)
	if err != nil {
		return nil, fmt.Errorf("create cluster: %w", err)
	}
	clusterID, _ := res.LastInsertId()

	for i, m := range members {
		order := m.MemberOrder
		if order == 0 {
			order = i
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO cluster_members (cluster_id, server_id, user, member_order) VALUES (?, ?, ?, ?)`,
			clusterID, m.ServerID, m.User, order); err != nil {
			return nil, fmt.Errorf("create cluster member: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, clusterID)
}

func (r *clusterRepo) GetByID(ctx context.Context, id int64) (*model.Cluster, error) {
	c := &model.Cluster{ID: id}
	if err := r.db.QueryRowContext(ctx,
		`SELECT name FROM clusters WHERE id = ?`, id).Scan(&c.Name); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.loadMembers(ctx, c)
}

func (r *clusterRepo) GetByName(ctx context.Context, name string) (*model.Cluster, error) {
	c := &model.Cluster{}
	if err := r.db.QueryRowContext(ctx,
		`SELECT id, name FROM clusters WHERE name = ?`, name).Scan(&c.ID, &c.Name); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.loadMembers(ctx, c)
}

func (r *clusterRepo) ListAll(ctx context.Context) ([]*model.Cluster, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name FROM clusters ORDER BY name`)
	if err != nil {
		return nil, err
	}

	var clusters []*model.Cluster
	for rows.Next() {
		c := &model.Cluster{}
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			rows.Close()
			return nil, err
		}
		clusters = append(clusters, c)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	for _, c := range clusters {
		if _, err := r.loadMembers(ctx, c); err != nil {
			return nil, err
		}
	}
	return clusters, nil
}

func (r *clusterRepo) Update(ctx context.Context, id int64, name string, members []model.ClusterMember) (*model.Cluster, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE clusters SET name = ? WHERE id = ?`, name, id); err != nil {
		return nil, fmt.Errorf("update cluster name: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM cluster_members WHERE cluster_id = ?`, id); err != nil {
		return nil, fmt.Errorf("clear cluster members: %w", err)
	}

	for i, m := range members {
		order := m.MemberOrder
		if order == 0 {
			order = i
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO cluster_members (cluster_id, server_id, user, member_order) VALUES (?, ?, ?, ?)`,
			id, m.ServerID, m.User, order); err != nil {
			return nil, fmt.Errorf("insert member %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *clusterRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM clusters WHERE id = ?`, id)
	return err
}

func (r *clusterRepo) loadMembers(ctx context.Context, c *model.Cluster) (*model.Cluster, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT server_id, user, member_order FROM cluster_members
		 WHERE cluster_id = ? ORDER BY member_order`, c.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		m := model.ClusterMember{}
		if err := rows.Scan(&m.ServerID, &m.User, &m.MemberOrder); err != nil {
			return nil, err
		}
		c.Members = append(c.Members, m)
	}
	return c, rows.Err()
}
