package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
	"stagecaption/internal/domain"
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func Open(path string) (*Store, error) {
	if path == "" {
		path = "stagecaption.db"
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite：%w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, now: time.Now}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, err
	}
	if err = s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

type Tx struct {
	tx           *sql.Tx
	store        *Store
	projectID    string
	nextRevision int64
	cuesChanged  bool
	findingsSet  bool
}

type Mutator func(*Tx, *domain.CaptionProject) (string, error)

func (s *Store) Write(ctx context.Context, projectID string, expected int64, actor, action string, allowBarrier bool, mutate Mutator) (domain.CaptionProject, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CaptionProject{}, err
	}
	defer tx.Rollback()
	p, err := getProjectTx(ctx, tx, projectID)
	if err != nil {
		return p, err
	}
	if p.Revision != expected {
		return p, &domain.ConflictError{CurrentRevision: p.Revision}
	}
	if err := p.Editable(); err != nil && action != "lock" {
		return p, err
	}
	if !allowBarrier {
		var barrier int
		if err := tx.QueryRowContext(ctx, "SELECT write_barrier FROM projects WHERE id=?", projectID).Scan(&barrier); err != nil {
			return p, err
		}
		if barrier != 0 {
			return p, domain.ErrWriteBarrier
		}
	}
	t := &Tx{tx: tx, store: s, projectID: projectID, nextRevision: p.Revision + 1}
	details, err := mutate(t, &p)
	if err != nil {
		return p, err
	}
	if !t.cuesChanged && !t.findingsSet {
		if _, err = tx.ExecContext(ctx, `UPDATE findings SET revision=? WHERE project_id=? AND revision=?`, t.nextRevision, projectID, p.Revision); err != nil {
			return p, err
		}
	}
	p.Revision++
	p.LastEditor = actor
	p.UpdatedAt = s.now().UTC()
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET title=?,production_version=?,frame_rate=?,duration_millis=?,time_origin=?,status=?,revision=?,last_editor=?,updated_at=? WHERE id=?`, p.Title, p.ProductionVersion, p.FrameRate, p.DurationMillis, p.TimeOrigin, p.Status, p.Revision, p.LastEditor, p.UpdatedAt.Format(time.RFC3339Nano), p.ID); err != nil {
		return p, err
	}
	if err = t.addAudit(ctx, p.Revision, actor, action, details); err != nil {
		return p, err
	}
	if err = t.saveSnapshot(ctx, p); err != nil {
		return p, err
	}
	if err = tx.Commit(); err != nil {
		return p, err
	}
	return p, nil
}

func getProjectTx(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (domain.CaptionProject, error) {
	var p domain.CaptionProject
	var status, created, updated string
	err := q.QueryRowContext(ctx, `SELECT id,title,production_version,frame_rate,duration_millis,time_origin,status,revision,last_editor,created_at,updated_at FROM projects WHERE id=?`, id).Scan(&p.ID, &p.Title, &p.ProductionVersion, &p.FrameRate, &p.DurationMillis, &p.TimeOrigin, &status, &p.Revision, &p.LastEditor, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Status = domain.ProjectStatus(status)
	createdAt, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return p, fmt.Errorf("解析项目创建时间失败：%w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return p, fmt.Errorf("解析项目更新时间失败：%w", err)
	}
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	return p, nil
}
