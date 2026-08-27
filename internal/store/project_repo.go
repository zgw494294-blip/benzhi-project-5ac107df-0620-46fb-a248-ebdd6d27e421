package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"stagecaption/internal/domain"
)

func (s *Store) CreateProject(ctx context.Context, p domain.CaptionProject) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO projects(id,title,production_version,frame_rate,duration_millis,time_origin,status,revision,last_editor,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Title, p.ProductionVersion, p.FrameRate, p.DurationMillis, p.TimeOrigin, p.Status, p.Revision, p.LastEditor, p.CreatedAt.Format(time.RFC3339Nano), p.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	t := &Tx{tx: tx, store: s, projectID: p.ID, nextRevision: p.Revision}
	if err = t.addAudit(ctx, p.Revision, p.LastEditor, "create", "建立项目与台本时间基准"); err != nil {
		return err
	}
	if err = t.saveSnapshot(ctx, p); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetProject(ctx context.Context, id string) (domain.CaptionProject, error) {
	return getProjectTx(ctx, s.db, id)
}

func (s *Store) ListProjects(ctx context.Context) ([]domain.CaptionProject, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM projects ORDER BY updated_at DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CaptionProject
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		p, e := s.GetProject(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type ProjectQueueRecord struct {
	Project                  domain.CaptionProject `json:"project"`
	CueCount                 int                   `json:"cueCount"`
	BlockingFindingCount     int                   `json:"blockingFindingCount"`
	UnresolvedBlockingIssues int                   `json:"unresolvedBlockingIssueCount"`
}

func (s *Store) ListProjectQueue(ctx context.Context) ([]ProjectQueueRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.title,p.production_version,p.frame_rate,p.duration_millis,p.time_origin,p.status,p.revision,p.last_editor,p.created_at,p.updated_at,
		(SELECT COUNT(*) FROM cues c WHERE c.project_id=p.id),
		(SELECT COUNT(*) FROM findings f WHERE f.project_id=p.id AND f.revision=p.revision AND f.severity='blocking'),
		(SELECT COUNT(*) FROM issues i WHERE i.project_id=p.id AND i.blocking=1 AND i.status='待整改')
		FROM projects p ORDER BY p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProjectQueueRecord{}
	for rows.Next() {
		var record ProjectQueueRecord
		var status, created, updated string
		p := &record.Project
		if err = rows.Scan(&p.ID, &p.Title, &p.ProductionVersion, &p.FrameRate, &p.DurationMillis, &p.TimeOrigin, &status, &p.Revision, &p.LastEditor, &created, &updated, &record.CueCount, &record.BlockingFindingCount, &record.UnresolvedBlockingIssues); err != nil {
			return nil, err
		}
		p.Status = domain.ProjectStatus(status)
		p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, record)
	}
	return out, rows.Err()
}

func (t *Tx) addAudit(ctx context.Context, revision int64, actor, action, details string) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO audits(project_id,revision,actor,action,details,created_at) VALUES(?,?,?,?,?,?)`, t.projectID, revision, actor, action, details, t.store.now().UTC().Format(time.RFC3339Nano))
	return err
}

func (t *Tx) saveSnapshot(ctx context.Context, p domain.CaptionProject) error {
	cues, err := listCuesQuery(ctx, t.tx, p.ID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(domain.RevisionSnapshot{Project: p, Cues: cues, CreatedAt: t.store.now().UTC()})
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO snapshots(project_id,revision,payload,created_at) VALUES(?,?,?,?)`, p.ID, p.Revision, payload, t.store.now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) SetBarrier(ctx context.Context, id string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
		s.barrierProject = id
	} else {
		id = s.barrierProject
		s.barrierProject = ""
	}
	res, err := s.db.ExecContext(ctx, `UPDATE projects SET write_barrier=? WHERE id=?`, v, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
} = (*sql.DB)(nil)
