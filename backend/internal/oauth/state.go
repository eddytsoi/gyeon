package oauth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// stateTTL bounds how long a started login may sit before the callback. Apple
// can take a few seconds (consent + form_post); 10 minutes is generous.
const stateTTL = 10 * time.Minute

type loginState struct {
	State        string
	Provider     string
	CodeVerifier string // Google PKCE
	Nonce        string // Apple id_token nonce
}

func (s *Service) saveState(ctx context.Context, st loginState) error {
	// Opportunistically sweep expired rows so the table stays small without a
	// separate cron.
	_, _ = s.db.ExecContext(ctx, `DELETE FROM oauth_login_states WHERE expires_at < NOW()`)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO oauth_login_states (state, provider, code_verifier, nonce, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		st.State, st.Provider, nullStr(st.CodeVerifier), nullStr(st.Nonce), time.Now().Add(stateTTL))
	return err
}

// consumeState atomically reads and deletes the row, so a state can be used at
// most once. Expired rows are treated as missing.
func (s *Service) consumeState(ctx context.Context, state string) (*loginState, error) {
	if state == "" {
		return nil, ErrInvalidState
	}
	var st loginState
	var verifier, nonce sql.NullString
	err := s.db.QueryRowContext(ctx,
		`DELETE FROM oauth_login_states
		   WHERE state=$1 AND expires_at > NOW()
		 RETURNING state, provider, code_verifier, nonce`, state).
		Scan(&st.State, &st.Provider, &verifier, &nonce)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidState
	}
	if err != nil {
		return nil, err
	}
	st.CodeVerifier = verifier.String
	st.Nonce = nonce.String
	return &st, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
