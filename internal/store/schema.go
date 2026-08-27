package store

import "context"

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS projects (id TEXT PRIMARY KEY,title TEXT NOT NULL,production_version TEXT NOT NULL,frame_rate REAL NOT NULL,duration_millis INTEGER NOT NULL,time_origin TEXT NOT NULL,status TEXT NOT NULL,revision INTEGER NOT NULL,last_editor TEXT NOT NULL,write_barrier INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL,updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS cues (id TEXT PRIMARY KEY,project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,scene TEXT NOT NULL,speaker TEXT NOT NULL,text TEXT NOT NULL,start_millis INTEGER NOT NULL,end_millis INTEGER NOT NULL,position INTEGER NOT NULL,revision INTEGER NOT NULL,updated_by TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS cues_project_time ON cues(project_id,start_millis,position)`,
		`CREATE TABLE IF NOT EXISTS findings (id TEXT PRIMARY KEY,project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,cue_id TEXT,rule_code TEXT NOT NULL,severity TEXT NOT NULL,message TEXT NOT NULL,observed_value TEXT NOT NULL,revision INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS rehearsals (id TEXT PRIMARY KEY,project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,recorder TEXT NOT NULL,notes TEXT NOT NULL,started_at TEXT NOT NULL,completed_at TEXT NOT NULL,revision INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS issues (id TEXT PRIMARY KEY,project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,rehearsal_id TEXT NOT NULL,cue_id TEXT NOT NULL,kind TEXT NOT NULL,blocking INTEGER NOT NULL,note TEXT NOT NULL,opened_revision INTEGER NOT NULL,resolved_revision INTEGER NOT NULL DEFAULT 0,resolution_note TEXT NOT NULL DEFAULT '',status TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS leases (project_id TEXT NOT NULL,scene TEXT NOT NULL,token TEXT NOT NULL,holder TEXT NOT NULL,expires_at TEXT NOT NULL,PRIMARY KEY(project_id,scene),FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS snapshots (project_id TEXT NOT NULL,revision INTEGER NOT NULL,payload BLOB NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(project_id,revision))`,
		`CREATE TABLE IF NOT EXISTS audits (id INTEGER PRIMARY KEY AUTOINCREMENT,project_id TEXT NOT NULL,revision INTEGER NOT NULL,actor TEXT NOT NULL,action TEXT NOT NULL,details TEXT NOT NULL,created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS releases (id TEXT PRIMARY KEY,project_id TEXT NOT NULL UNIQUE REFERENCES projects(id),locked_revision INTEGER NOT NULL,webvtt_digest TEXT NOT NULL,manifest_digest TEXT NOT NULL,credential_digest TEXT NOT NULL,reviewer TEXT NOT NULL,issued_at TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	// 兼容早期数据库；重复列是唯一可忽略的迁移结果。
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE findings ADD COLUMN revision INTEGER NOT NULL DEFAULT 0`); err != nil {
		var count int
		if checkErr := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('findings') WHERE name='revision'`).Scan(&count); checkErr != nil || count != 1 {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE findings SET revision=(SELECT revision FROM projects WHERE projects.id=findings.project_id) WHERE revision=0`); err != nil {
		return err
	}
	return nil
}
