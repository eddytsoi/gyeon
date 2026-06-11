package admin

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/lib/pq"
)

// DashboardPrefsService persists per-admin dashboard customisation: named layout
// presets plus the active selection and global compare mode. All operations are
// scoped to a single admin (the caller passes the JWT subject's id).
type DashboardPrefsService struct{ db *sql.DB }

func NewDashboardPrefsService(db *sql.DB) *DashboardPrefsService {
	return &DashboardPrefsService{db: db}
}

// DashboardPreset is one saved layout. Layout is opaque JSON owned by the
// frontend (sections → widgets → visible/order).
type DashboardPreset struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	IsDefault bool            `json:"is_default"`
	Layout    json.RawMessage `json:"layout"`
}

type DashboardPrefs struct {
	Presets        []DashboardPreset `json:"presets"`
	ActivePresetID *string           `json:"active_preset_id"`
	CompareMode    string            `json:"compare_mode"`
}

// Get returns the admin's presets + active selection + compare mode. A fresh
// admin has no rows yet → presets is empty and compare defaults to prev_month;
// the frontend then renders its registry default and only persists on first edit.
func (s *DashboardPrefsService) Get(ctx context.Context, adminID string) (DashboardPrefs, error) {
	out := DashboardPrefs{Presets: []DashboardPreset{}, CompareMode: "prev_month"}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, is_default, layout FROM admin_dashboard_layouts
		  WHERE admin_id = $1 ORDER BY sort_order, created_at`, adminID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var p DashboardPreset
		var layout []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.IsDefault, &layout); err != nil {
			return out, err
		}
		p.Layout = json.RawMessage(layout)
		out.Presets = append(out.Presets, p)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	var active sql.NullString
	var compare sql.NullString
	if err := s.db.QueryRowContext(ctx,
		`SELECT active_layout_id, dashboard_compare_mode FROM admin_users WHERE id = $1`, adminID).
		Scan(&active, &compare); err != nil && err != sql.ErrNoRows {
		return out, err
	}
	if active.Valid {
		out.ActivePresetID = &active.String
	}
	if compare.Valid && compare.String != "" {
		out.CompareMode = compare.String
	}
	return out, nil
}

// Save transactionally replaces the admin's preset set with the supplied list
// (deleting presets no longer present, upserting the rest by client-provided id)
// and updates the active selection + compare mode. The whole-set replace keeps
// the frontend simple: it holds the entire layout in memory and PUTs the lot.
func (s *DashboardPrefsService) Save(ctx context.Context, adminID string, prefs DashboardPrefs) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	keepIDs := make([]string, 0, len(prefs.Presets))
	for _, p := range prefs.Presets {
		if p.ID != "" {
			keepIDs = append(keepIDs, p.ID)
		}
	}

	// Drop the active pointer first so deleting its target doesn't trip ordering.
	if _, err := tx.ExecContext(ctx, `UPDATE admin_users SET active_layout_id = NULL WHERE id = $1`, adminID); err != nil {
		return err
	}

	if len(keepIDs) > 0 {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM admin_dashboard_layouts WHERE admin_id = $1 AND NOT (id = ANY($2::uuid[]))`,
			adminID, pq.Array(keepIDs)); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `DELETE FROM admin_dashboard_layouts WHERE admin_id = $1`, adminID); err != nil {
			return err
		}
	}

	for i, p := range prefs.Presets {
		layout := p.Layout
		if len(layout) == 0 {
			layout = json.RawMessage("{}")
		}
		if p.ID == "" {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO admin_dashboard_layouts (admin_id, name, is_default, layout, sort_order)
				 VALUES ($1, $2, $3, $4, $5)`,
				adminID, p.Name, p.IsDefault, []byte(layout), i); err != nil {
				return err
			}
		} else if _, err := tx.ExecContext(ctx,
			`INSERT INTO admin_dashboard_layouts (id, admin_id, name, is_default, layout, sort_order)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (id) DO UPDATE
			   SET name = EXCLUDED.name, is_default = EXCLUDED.is_default,
			       layout = EXCLUDED.layout, sort_order = EXCLUDED.sort_order`,
			p.ID, adminID, p.Name, p.IsDefault, []byte(layout), i); err != nil {
			return err
		}
	}

	compare := prefs.CompareMode
	switch compare {
	case "prev_month", "prev_period", "prev_year", "none":
	default:
		compare = "prev_month"
	}
	var active any
	if prefs.ActivePresetID != nil && *prefs.ActivePresetID != "" {
		active = *prefs.ActivePresetID
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE admin_users SET active_layout_id = $2, dashboard_compare_mode = $3 WHERE id = $1`,
		adminID, active, compare); err != nil {
		return err
	}

	return tx.Commit()
}
