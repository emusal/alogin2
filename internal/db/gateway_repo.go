package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/emusal/alogin2/internal/model"
)

// GatewayRepo defines CRUD operations for gateway routes.
type GatewayRepo interface {
	Create(ctx context.Context, name string, hopServerIDs []int64) (*model.GatewayRoute, error)
	GetByName(ctx context.Context, name string) (*model.GatewayRoute, error)
	GetByID(ctx context.Context, id int64) (*model.GatewayRoute, error)
	// HopsFor returns the ordered hop chain for a server's gateway route.
	HopsFor(ctx context.Context, serverID int64) ([]model.GatewayHop, error)
	ListAll(ctx context.Context) ([]*model.GatewayRoute, error)
	Update(ctx context.Context, id int64, name string, hopServerIDs []int64) (*model.GatewayRoute, error)
	Delete(ctx context.Context, id int64) error
}

type gatewayRepo struct{ db *sql.DB }

func (r *gatewayRepo) Create(ctx context.Context, name string, hopServerIDs []int64) (*model.GatewayRoute, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `INSERT INTO gateway_routes (name) VALUES (?)`, name)
	if err != nil {
		return nil, fmt.Errorf("create gateway route: %w", err)
	}
	routeID, _ := res.LastInsertId()

	for i, sid := range hopServerIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO gateway_hops (route_id, server_id, hop_order) VALUES (?, ?, ?)`,
			routeID, sid, i); err != nil {
			return nil, fmt.Errorf("create gateway hop: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, routeID)
}

func (r *gatewayRepo) GetByName(ctx context.Context, name string) (*model.GatewayRoute, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id FROM gateway_routes WHERE name = ?`, name)
	var id int64
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *gatewayRepo) GetByID(ctx context.Context, id int64) (*model.GatewayRoute, error) {
	route := &model.GatewayRoute{ID: id}
	if err := r.db.QueryRowContext(ctx, `SELECT name FROM gateway_routes WHERE id = ?`, id).
		Scan(&route.Name); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	hops, err := r.loadHops(ctx, id)
	if err != nil {
		return nil, err
	}
	route.Hops = hops
	return route, nil
}

func (r *gatewayRepo) HopsFor(ctx context.Context, serverID int64) ([]model.GatewayHop, error) {
	var routeID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT gateway_id FROM servers WHERE id = ?`, serverID).Scan(&routeID)
	if err != nil || !routeID.Valid {
		return nil, err
	}
	return r.loadHops(ctx, routeID.Int64)
}

func (r *gatewayRepo) ListAll(ctx context.Context) ([]*model.GatewayRoute, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name FROM gateway_routes ORDER BY name`)
	if err != nil {
		return nil, err
	}

	var routes []*model.GatewayRoute
	for rows.Next() {
		rt := &model.GatewayRoute{}
		if err := rows.Scan(&rt.ID, &rt.Name); err != nil {
			rows.Close()
			return nil, err
		}
		routes = append(routes, rt)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	// Load hops after closing rows — avoids deadlock with SetMaxOpenConns(1)
	for _, rt := range routes {
		rt.Hops, err = r.loadHops(ctx, rt.ID)
		if err != nil {
			return nil, err
		}
	}
	return routes, nil
}

func (r *gatewayRepo) Update(ctx context.Context, id int64, name string, hopServerIDs []int64) (*model.GatewayRoute, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE gateway_routes SET name = ? WHERE id = ?`, name, id); err != nil {
		return nil, fmt.Errorf("update gateway name: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM gateway_hops WHERE route_id = ?`, id); err != nil {
		return nil, fmt.Errorf("clear gateway hops: %w", err)
	}

	for i, sid := range hopServerIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO gateway_hops (route_id, server_id, hop_order) VALUES (?, ?, ?)`,
			id, sid, i); err != nil {
			return nil, fmt.Errorf("insert hop %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *gatewayRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM gateway_routes WHERE id = ?`, id)
	return err
}

func (r *gatewayRepo) loadHops(ctx context.Context, routeID int64) ([]model.GatewayHop, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT server_id, hop_order FROM gateway_hops WHERE route_id = ? ORDER BY hop_order`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hops []model.GatewayHop
	for rows.Next() {
		var h model.GatewayHop
		if err := rows.Scan(&h.ServerID, &h.HopOrder); err != nil {
			return nil, err
		}
		hops = append(hops, h)
	}
	return hops, rows.Err()
}
