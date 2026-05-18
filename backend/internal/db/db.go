package db

import (
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	// Bound the pool so a request pile-up can't exhaust Postgres
	// (default max_connections=100). Lifetime + IdleTime recycle stale
	// connections that may have been silently dropped by the network or by
	// Postgres' own idle-close — common cause of "500 after a quiet period".
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}
