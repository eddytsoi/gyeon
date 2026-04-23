package settings

import (
	"context"
	"database/sql"
)

type Setting struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
	UpdatedAt   string  `json:"updated_at"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context) ([]Setting, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value, description, updated_at FROM site_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make([]Setting, 0)
	for rows.Next() {
		var st Setting
		if err := rows.Scan(&st.Key, &st.Value, &st.Description, &st.UpdatedAt); err != nil {
			return nil, err
		}
		settings = append(settings, st)
	}
	return settings, rows.Err()
}

func (s *Service) Get(ctx context.Context, key string) (*Setting, error) {
	var st Setting
	err := s.db.QueryRowContext(ctx,
		`SELECT key, value, description, updated_at FROM site_settings WHERE key=$1`, key).
		Scan(&st.Key, &st.Value, &st.Description, &st.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Service) Set(ctx context.Context, key, value string) (*Setting, error) {
	var st Setting
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO site_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()
		 RETURNING key, value, description, updated_at`,
		key, value).
		Scan(&st.Key, &st.Value, &st.Description, &st.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Service) BulkSet(ctx context.Context, updates map[string]string) ([]Setting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for key, value := range updates {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO site_settings (key, value) VALUES ($1, $2)
			 ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()`,
			key, value)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.List(ctx)
}
