package malformedprojecttimestamp_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"stagecaption/internal/domain"
	"stagecaption/internal/store"
)

func TestMalformedProjectTimestampMustSurfaceStorageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timestamp.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	project, err := domain.NewProject("project-one", "演出", "v1", 25, 6000, "开场", "编辑", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = st.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(context.Background(), "UPDATE projects SET created_at=?, updated_at=? WHERE id=?", "not-a-timestamp", "also-invalid", project.ID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	st, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	loaded, err := st.GetProject(context.Background(), project.ID)
	if err == nil {
		t.Fatalf("损坏的持久化时间戳被静默转换为零值：createdAt=%s updatedAt=%s", loaded.CreatedAt, loaded.UpdatedAt)
	}
}
