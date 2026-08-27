package store

import (
	"context"
	"stagecaption/internal/domain"
	"time"
)

func (t *Tx) AddRehearsal(ctx context.Context, r domain.Rehearsal, issues []domain.RehearsalIssue) error {
	r.Revision = t.nextRevision
	_, err := t.tx.ExecContext(ctx, `INSERT INTO rehearsals(id,project_id,recorder,notes,started_at,completed_at,revision) VALUES(?,?,?,?,?,?,?)`, r.ID, r.ProjectID, r.Recorder, r.Notes, r.StartedAt.Format(time.RFC3339Nano), r.CompletedAt.Format(time.RFC3339Nano), r.Revision)
	if err != nil {
		return err
	}
	for _, i := range issues {
		_, err = t.tx.ExecContext(ctx, `INSERT INTO issues(id,project_id,rehearsal_id,cue_id,kind,blocking,note,opened_revision,resolved_revision,resolution_note,status) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, i.ID, i.ProjectID, i.RehearsalID, i.CueID, i.Kind, i.Blocking, i.Note, i.OpenedAgainstRevision, i.ResolvedByRevision, i.ResolutionNote, i.Status)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tx) ResolveIssue(ctx context.Context, id, note string) error {
	res, err := t.tx.ExecContext(ctx, `UPDATE issues SET resolved_revision=?,resolution_note=?,status='已解决' WHERE id=? AND project_id=? AND status='待整改'`, t.nextRevision, note, id, t.projectID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ListIssues(ctx context.Context, projectID string) ([]domain.RehearsalIssue, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,rehearsal_id,cue_id,kind,blocking,note,opened_revision,resolved_revision,resolution_note,status FROM issues WHERE project_id=? ORDER BY opened_revision,id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.RehearsalIssue{}
	for rows.Next() {
		var i domain.RehearsalIssue
		if err = rows.Scan(&i.ID, &i.ProjectID, &i.RehearsalID, &i.CueID, &i.Kind, &i.Blocking, &i.Note, &i.OpenedAgainstRevision, &i.ResolvedByRevision, &i.ResolutionNote, &i.Status); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) ListRehearsals(ctx context.Context, projectID string) ([]domain.Rehearsal, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,recorder,notes,started_at,completed_at,revision FROM rehearsals WHERE project_id=? ORDER BY revision`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Rehearsal{}
	for rows.Next() {
		var r domain.Rehearsal
		var a, b string
		if err = rows.Scan(&r.ID, &r.ProjectID, &r.Recorder, &r.Notes, &a, &b, &r.Revision); err != nil {
			return nil, err
		}
		r.StartedAt, _ = time.Parse(time.RFC3339Nano, a)
		r.CompletedAt, _ = time.Parse(time.RFC3339Nano, b)
		out = append(out, r)
	}
	return out, rows.Err()
}
