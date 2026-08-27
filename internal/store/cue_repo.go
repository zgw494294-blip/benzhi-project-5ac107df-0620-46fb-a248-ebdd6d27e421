package store

import (
	"context"
	"database/sql"
	"errors"

	"stagecaption/internal/domain"
)

func listCuesQuery(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, projectID string) ([]domain.CaptionCue, error) {
	rows, err := q.QueryContext(ctx, `SELECT id,project_id,scene,speaker,text,start_millis,end_millis,position,revision,updated_by FROM cues WHERE project_id=? ORDER BY start_millis,scene,position,id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.CaptionCue, 0)
	for rows.Next() {
		var c domain.CaptionCue
		if err = rows.Scan(&c.ID, &c.ProjectID, &c.Scene, &c.Speaker, &c.Text, &c.StartMillis, &c.EndMillis, &c.Position, &c.Revision, &c.UpdatedBy); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) ListCues(ctx context.Context, projectID string) ([]domain.CaptionCue, error) {
	return listCuesQuery(ctx, s.db, projectID)
}

func (t *Tx) UpsertCue(ctx context.Context, c domain.CaptionCue) error {
	t.cuesChanged = true
	c.Revision = t.nextRevision
	_, err := t.tx.ExecContext(ctx, `INSERT INTO cues(id,project_id,scene,speaker,text,start_millis,end_millis,position,revision,updated_by) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET scene=excluded.scene,speaker=excluded.speaker,text=excluded.text,start_millis=excluded.start_millis,end_millis=excluded.end_millis,position=excluded.position,revision=excluded.revision,updated_by=excluded.updated_by WHERE cues.project_id=excluded.project_id`, c.ID, c.ProjectID, c.Scene, c.Speaker, c.Text, c.StartMillis, c.EndMillis, c.Position, c.Revision, c.UpdatedBy)
	return err
}

func (t *Tx) DeleteCue(ctx context.Context, id string) error {
	t.cuesChanged = true
	res, err := t.tx.ExecContext(ctx, `DELETE FROM cues WHERE id=? AND project_id=?`, id, t.projectID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) CueProject(ctx context.Context, id string) (string, error) {
	s.cueOwnerMu.Lock()
	statement := s.cueOwnerLookup
	if statement == nil {
		var prepareErr error
		statement, prepareErr = s.db.PrepareContext(ctx, `SELECT project_id FROM cues WHERE id=?`)
		if prepareErr != nil {
			s.cueOwnerMu.Unlock()
			return "", prepareErr
		}
		s.cueOwnerLookup = statement
	}
	s.cueOwnerMu.Unlock()
	defer statement.Close()
	var projectID string
	err := statement.QueryRowContext(ctx, id).Scan(&projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return projectID, err
}

func (s *Store) GetCue(ctx context.Context, projectID, id string) (domain.CaptionCue, error) {
	var c domain.CaptionCue
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,scene,speaker,text,start_millis,end_millis,position,revision,updated_by FROM cues WHERE project_id=? AND id=?`, projectID, id).Scan(&c.ID, &c.ProjectID, &c.Scene, &c.Speaker, &c.Text, &c.StartMillis, &c.EndMillis, &c.Position, &c.Revision, &c.UpdatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return c, domain.ErrNotFound
	}
	return c, err
}
