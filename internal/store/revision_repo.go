package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"stagecaption/internal/domain"
)

func (s *Store) GetSnapshot(ctx context.Context, projectID string, revision int64) (domain.RevisionSnapshot, error) {
	var snap domain.RevisionSnapshot
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM snapshots WHERE project_id=? AND revision=?`, projectID, revision).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return snap, domain.ErrNotFound
	}
	if err != nil {
		return snap, err
	}
	err = json.Unmarshal(payload, &snap)
	return snap, err
}

type RevisionDiff struct {
	FromRevision int64               `json:"fromRevision"`
	ToRevision   int64               `json:"toRevision"`
	Added        []domain.CaptionCue `json:"added"`
	Changed      []domain.CaptionCue `json:"changed"`
	Removed      []domain.CaptionCue `json:"removed"`
}

func (s *Store) Diff(ctx context.Context, projectID string, from, to int64) (RevisionDiff, error) {
	a, err := s.GetSnapshot(ctx, projectID, from)
	if err != nil {
		return RevisionDiff{}, err
	}
	b, err := s.GetSnapshot(ctx, projectID, to)
	if err != nil {
		return RevisionDiff{}, err
	}
	d := RevisionDiff{FromRevision: from, ToRevision: to}
	old := map[string]domain.CaptionCue{}
	for _, c := range a.Cues {
		old[c.ID] = c
	}
	for _, c := range b.Cues {
		previous, ok := old[c.ID]
		if !ok {
			d.Added = append(d.Added, c)
		} else if previous.Text != c.Text || previous.StartMillis != c.StartMillis || previous.EndMillis != c.EndMillis || previous.Scene != c.Scene || previous.Speaker != c.Speaker {
			d.Changed = append(d.Changed, c)
		}
		delete(old, c.ID)
	}
	for _, c := range old {
		d.Removed = append(d.Removed, c)
	}
	domain.SortCues(d.Removed)
	return d, nil
}
