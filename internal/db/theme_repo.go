package db

import (
	"context"
	"database/sql"
	"regexp"
	"sort"

	"github.com/emusal/alogin2/internal/model"
)

// ThemeRepo looks up terminal themes by locale or hostname.
type ThemeRepo interface {
	Create(ctx context.Context, t *model.TermTheme) error
	Resolve(ctx context.Context, host, locale string) (string, error)
	ListAll(ctx context.Context) ([]*model.TermTheme, error)
	Delete(ctx context.Context, id int64) error
}

type themeRepo struct{ db *sql.DB }

func (r *themeRepo) Create(ctx context.Context, t *model.TermTheme) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO term_themes (locale_pattern, host_pattern, theme_name, priority)
		 VALUES (?, ?, ?, ?)`,
		t.LocalePattern, t.HostPattern, t.ThemeName, t.Priority)
	return err
}

// Resolve returns the terminal theme name for the given host+locale.
// host_pattern takes priority over locale_pattern; higher priority value wins.
func (r *themeRepo) Resolve(ctx context.Context, host, locale string) (string, error) {
	themes, err := r.ListAll(ctx)
	if err != nil {
		return "", err
	}

	// Sort by priority descending
	sort.Slice(themes, func(i, j int) bool {
		return themes[i].Priority > themes[j].Priority
	})

	for _, t := range themes {
		if t.HostPattern != "" {
			if ok, _ := regexp.MatchString(t.HostPattern, host); ok {
				return t.ThemeName, nil
			}
		}
	}
	for _, t := range themes {
		if t.LocalePattern != "" {
			if ok, _ := regexp.MatchString(t.LocalePattern, locale); ok {
				return t.ThemeName, nil
			}
		}
	}
	return "", nil
}

func (r *themeRepo) ListAll(ctx context.Context) ([]*model.TermTheme, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, locale_pattern, host_pattern, theme_name, priority FROM term_themes ORDER BY priority DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var themes []*model.TermTheme
	for rows.Next() {
		t := &model.TermTheme{}
		if err := rows.Scan(&t.ID, &t.LocalePattern, &t.HostPattern, &t.ThemeName, &t.Priority); err != nil {
			return nil, err
		}
		themes = append(themes, t)
	}
	return themes, rows.Err()
}

func (r *themeRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM term_themes WHERE id = ?`, id)
	return err
}

// ensure interface satisfied
var _ ThemeRepo = (*themeRepo)(nil)
var _ sql.NullInt64 = sql.NullInt64{}
