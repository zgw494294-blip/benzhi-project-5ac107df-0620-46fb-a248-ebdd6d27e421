package workspacecontextdetach_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"stagecaption/internal/quality"
	"stagecaption/internal/service"
	"stagecaption/internal/store"
	"stagecaption/internal/web"
)

func TestCanceledWorkspaceRequestMustNotReturnSuccess(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "workspace-context.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := service.New(st, quality.New())
	project, err := svc.CreateProject(context.Background(), service.CreateProjectInput{
		Title: "取消传播复现", ProductionVersion: "v1", FrameRate: 25,
		DurationMillis: 6000, TimeOrigin: "开场", Actor: "编辑甲",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID, nil).WithContext(ctx)
	response := httptest.NewRecorder()
	web.New(svc).ServeHTTP(response, request)

	if response.Code == http.StatusOK {
		t.Fatalf("已取消的工作区请求仍完成全部聚合查询并返回 200")
	}
}
