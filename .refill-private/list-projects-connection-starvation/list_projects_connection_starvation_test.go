package listprojectsstarvation_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"stagecaption/internal/domain"
	"stagecaption/internal/store"
)

func TestListProjectsDoesNotStarveItsOwnConnection(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "projects.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := domain.NewProject("project-one", "演出", "v1", 25, 6000, "开场", "编辑", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = st.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	projects, err := st.ListProjects(ctx)
	if err != nil {
		t.Fatalf("列出非空项目时耗尽自身数据库连接：%v", err)
	}
	if len(projects) != 1 || projects[0].ID != project.ID {
		t.Fatalf("项目列表异常：%+v", projects)
	}
}
