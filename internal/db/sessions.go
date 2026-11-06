package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

func scanAuthSession(scanner interface {
	Scan(dest ...any) error
}) (*domain.AuthSession, error) {
	var (
		sess     domain.AuthSession
		remember int
		revoked  sql.NullTime
	)
	if err := scanner.Scan(
		&sess.ID,
		&sess.UserID,
		&sess.CreatedAt,
		&sess.LastSeenAt,
		&sess.ExpiresAt,
		&revoked,
		&remember,
		&sess.UserAgent,
		&sess.IPAddress,
	); err != nil {
		return nil, err
	}
	sess.RememberMe = remember == 1
	if revoked.Valid {
		t := revoked.Time
		sess.RevokedAt = &t
	}
	return &sess, nil
}

func (s *Store) CreateAuthSession(sess *domain.AuthSession) error {
	if sess == nil || sess.ID == "" || sess.UserID <= 0 {
		return errors.New("invalid auth session")
	}
	remember := 0
	if sess.RememberMe {
		remember = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO auth_sessions (
			id, user_id, created_at, last_seen_at, expires_at, remember_me, user_agent, ip_address
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID,
		sess.UserID,
		sess.CreatedAt.UTC(),
		sess.LastSeenAt.UTC(),
		sess.ExpiresAt.UTC(),
		remember,
		sess.UserAgent,
		sess.IPAddress,
	)
	if err != nil {
		return fmt.Errorf("create auth session: %w", err)
	}
	return nil
}

func (s *Store) GetAuthSession(id string) (*domain.AuthSession, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, created_at, last_seen_at, expires_at, revoked_at, remember_me, user_agent, ip_address
		FROM auth_sessions WHERE id = ?`, id)
	sess, err := scanAuthSession(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("get auth session: %w", err)
	}
	return sess, nil
}

func (s *Store) ListAuthSessionsForUser(userID int64) ([]domain.AuthSession, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, created_at, last_seen_at, expires_at, revoked_at, remember_me, user_agent, ip_address
		FROM auth_sessions
		WHERE user_id = ? AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP
		ORDER BY last_seen_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list auth sessions: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AuthSession, 0)
	for rows.Next() {
		sess, err := scanAuthSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sess)
	}
	return out, rows.Err()
}

func (s *Store) TouchAuthSession(id string, at time.Time) error {
	_, err := s.db.Exec(`UPDATE auth_sessions SET last_seen_at = ? WHERE id = ? AND revoked_at IS NULL`, at.UTC(), id)
	if err != nil {
		return fmt.Errorf("touch auth session: %w", err)
	}
	return nil
}

func (s *Store) RevokeAuthSession(id string, at time.Time) error {
	res, err := s.db.Exec(`
		UPDATE auth_sessions SET revoked_at = ?
		WHERE id = ? AND revoked_at IS NULL`, at.UTC(), id)
	if err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RevokeOtherAuthSessions(userID int64, keepID string, at time.Time) (int64, error) {
	res, err := s.db.Exec(`
		UPDATE auth_sessions SET revoked_at = ?
		WHERE user_id = ? AND id != ? AND revoked_at IS NULL`, at.UTC(), userID, keepID)
	if err != nil {
		return 0, fmt.Errorf("revoke other auth sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *Store) DeleteExpiredAuthSessions(before time.Time) error {
	_, err := s.db.Exec(`
		DELETE FROM auth_sessions
		WHERE expires_at < ? OR (revoked_at IS NOT NULL AND revoked_at < ?)`,
		before.UTC(), before.Add(-7*24*time.Hour).UTC())
	if err != nil {
		return fmt.Errorf("delete expired auth sessions: %w", err)
	}
	return nil
}
