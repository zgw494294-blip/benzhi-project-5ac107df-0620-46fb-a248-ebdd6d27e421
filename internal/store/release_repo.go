package store

import (
	"context"
	"database/sql"
	"errors"
	"stagecaption/internal/domain"
	"time"
)

func (t *Tx) SaveRelease(ctx context.Context, r domain.ReleaseBundle) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO releases(id,project_id,locked_revision,webvtt_digest,manifest_digest,credential_digest,reviewer,issued_at) VALUES(?,?,?,?,?,?,?,?)`, r.ID, r.ProjectID, r.LockedRevision, r.WebVTTDigest, r.ManifestDigest, r.CredentialDigest, r.Reviewer, r.IssuedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetRelease(ctx context.Context, projectID string) (domain.ReleaseBundle, error) {
	var r domain.ReleaseBundle
	var issued string
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,locked_revision,webvtt_digest,manifest_digest,credential_digest,reviewer,issued_at FROM releases WHERE project_id=?`, projectID).Scan(&r.ID, &r.ProjectID, &r.LockedRevision, &r.WebVTTDigest, &r.ManifestDigest, &r.CredentialDigest, &r.Reviewer, &issued)
	if errors.Is(err, sql.ErrNoRows) {
		return r, domain.ErrNotFound
	}
	r.IssuedAt, _ = time.Parse(time.RFC3339Nano, issued)
	return r, err
}

func (s *Store) ListAudits(ctx context.Context, projectID string) ([]domain.AuditRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,revision,actor,action,details,created_at FROM audits WHERE project_id=? ORDER BY id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AuditRecord{}
	for rows.Next() {
		var a domain.AuditRecord
		var created string
		if err = rows.Scan(&a.ID, &a.ProjectID, &a.Revision, &a.Actor, &a.Action, &a.Details, &created); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, a)
	}
	return out, rows.Err()
}
