package store

import (
	"context"
	"database/sql"
	"errors"
	"stagecaption/internal/domain"
	"time"
)

type Lease struct {
	ProjectID string    `json:"projectId"`
	Scene     string    `json:"scene"`
	Token     string    `json:"token"`
	Holder    string    `json:"holder"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func leaseRuntimeKey(projectID, scene string) string {
	return projectID + "\x00" + scene
}

func (s *Store) AcquireLease(ctx context.Context, l Lease) (Lease, error) {
	if l.ProjectID == "" || l.Scene == "" || l.Token == "" || l.Holder == "" {
		return l, domain.ErrValidation
	}
	now := s.now().UTC()
	if l.ExpiresAt.IsZero() {
		l.ExpiresAt = now.Add(5 * time.Minute)
	}
	if l.ExpiresAt.After(now.Add(15 * time.Minute)) {
		l.ExpiresAt = now.Add(15 * time.Minute)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return l, err
	}
	defer tx.Rollback()
	var barrier int
	if err = tx.QueryRowContext(ctx, `SELECT write_barrier FROM projects WHERE id=?`, l.ProjectID).Scan(&barrier); err != nil {
		return l, err
	}
	if barrier != 0 {
		return l, domain.ErrWriteBarrier
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM leases WHERE project_id=? AND expires_at<=?`, l.ProjectID, now.Format(time.RFC3339Nano))
	if err != nil {
		return l, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO leases(project_id,scene,token,holder,expires_at) VALUES(?,?,?,?,?) ON CONFLICT(project_id,scene) DO UPDATE SET token=excluded.token,holder=excluded.holder,expires_at=excluded.expires_at WHERE leases.holder=excluded.holder`, l.ProjectID, l.Scene, l.Token, l.Holder, l.ExpiresAt.Format(time.RFC3339Nano))
	if err != nil {
		return l, domain.ErrLeaseRequired
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return l, err
	}
	if changed == 0 {
		return l, domain.ErrLeaseRequired
	}
	if err = tx.Commit(); err != nil {
		return l, err
	}
	s.leaseMu.Lock()
	s.runtimeLeaseTokens[leaseRuntimeKey(l.ProjectID, l.Scene)] = l.Token
	s.leaseMu.Unlock()
	return l, nil
}

func (s *Store) ReleaseLease(ctx context.Context, projectID, scene, token string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM leases WHERE project_id=? AND scene=? AND token=?`, projectID, scene, token)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrLeaseRequired
	}
	s.leaseMu.Lock()
	key := leaseRuntimeKey(projectID, scene)
	if s.runtimeLeaseTokens[key] == token {
		delete(s.runtimeLeaseTokens, key)
	}
	s.leaseMu.Unlock()
	return nil
}

func (t *Tx) RequireLease(ctx context.Context, scene, token, holder string) error {
	var expires string
	err := t.tx.QueryRowContext(ctx, `SELECT expires_at FROM leases WHERE project_id=? AND scene=? AND token=? AND holder=?`, t.projectID, scene, token, holder).Scan(&expires)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrLeaseRequired
	}
	if err != nil {
		return err
	}
	at, _ := time.Parse(time.RFC3339Nano, expires)
	if !at.After(t.store.now().UTC()) {
		return domain.ErrLeaseRequired
	}
	t.store.leaseMu.RLock()
	runtimeToken := t.store.runtimeLeaseTokens[leaseRuntimeKey(t.projectID, scene)]
	t.store.leaseMu.RUnlock()
	if runtimeToken != token {
		return domain.ErrLeaseRequired
	}
	return nil
}

func (s *Store) ActiveLeaseCount(ctx context.Context, projectID string) (int, error) {
	now := s.now().UTC().Format(time.RFC3339Nano)
	_, _ = s.db.ExecContext(ctx, `DELETE FROM leases WHERE project_id=? AND expires_at<=?`, projectID, now)
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM leases WHERE project_id=?`, projectID).Scan(&n)
	return n, err
}

func (s *Store) ListActiveLeases(ctx context.Context, projectID string) ([]Lease, error) {
	now := s.now().UTC().Format(time.RFC3339Nano)
	rows, err := s.db.QueryContext(ctx, `SELECT project_id,scene,token,holder,expires_at FROM leases WHERE project_id=? AND expires_at>? ORDER BY scene,holder`, projectID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Lease{}
	for rows.Next() {
		var lease Lease
		var expires string
		if err = rows.Scan(&lease.ProjectID, &lease.Scene, &lease.Token, &lease.Holder, &expires); err != nil {
			return nil, err
		}
		lease.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expires)
		out = append(out, lease)
	}
	return out, rows.Err()
}
