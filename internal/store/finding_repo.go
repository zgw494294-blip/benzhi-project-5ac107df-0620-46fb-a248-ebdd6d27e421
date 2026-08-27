package store

import (
	"context"
	"database/sql"
	"stagecaption/internal/domain"
	"time"
)

func (t *Tx) ReplaceFindings(ctx context.Context, findings []domain.QualityFinding) error {
	t.findingsSet = true
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM findings WHERE project_id=?`, t.projectID); err != nil {
		return err
	}
	for _, f := range findings {
		_, err := t.tx.ExecContext(ctx, `INSERT INTO findings(id,project_id,cue_id,rule_code,severity,message,observed_value,revision,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, f.ID, f.ProjectID, f.CueID, f.RuleCode, f.Severity, f.Message, f.ObservedValue, t.nextRevision, f.CreatedAt.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListFindings(ctx context.Context, projectID string) ([]domain.QualityFinding, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT f.id,f.project_id,f.cue_id,f.rule_code,f.severity,f.message,f.observed_value,f.created_at FROM findings f JOIN projects p ON p.id=f.project_id WHERE f.project_id=? AND f.revision=p.revision ORDER BY f.rule_code,f.cue_id,f.id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.QualityFinding{}
	for rows.Next() {
		var f domain.QualityFinding
		var created string
		if err = rows.Scan(&f.ID, &f.ProjectID, &f.CueID, &f.RuleCode, &f.Severity, &f.Message, &f.ObservedValue, &created); err != nil {
			return nil, err
		}
		f.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, f)
	}
	return out, rows.Err()
}

var _ *sql.Rows
