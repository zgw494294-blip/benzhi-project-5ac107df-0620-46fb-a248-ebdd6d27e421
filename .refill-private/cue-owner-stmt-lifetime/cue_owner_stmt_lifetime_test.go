package cueownerstmtlifetime_test

import (
	"context"
	"path/filepath"
	"testing"

	"stagecaption/internal/domain"
	"stagecaption/internal/quality"
	"stagecaption/internal/service"
	"stagecaption/internal/store"
)

func TestBatchExplicitIDsMustNotReuseClosedStatement(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "cue-owner-stmt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := service.New(st, quality.New())
	ctx := context.Background()
	project, err := svc.CreateProject(ctx, service.CreateProjectInput{
		Title: "查询资源复现", ProductionVersion: "v1", FrameRate: 25,
		DurationMillis: 6000, TimeOrigin: "开场", Actor: "编辑甲",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := svc.AcquireLease(ctx, project.ID, service.LeaseInput{Scene: "第一场", Actor: "编辑甲"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := svc.BatchUpsertCues(ctx, project.ID, service.BatchCueInput{
		Scene: "第一场", Actor: "编辑甲", LeaseToken: token, ExpectedRevision: project.Revision,
		Rows: []domain.BatchCueRow{
			{ID: "explicit-cue-a", Speaker: "甲", Text: "第一条", StartMillis: 0, EndMillis: 3000, Position: 1},
			{ID: "explicit-cue-b", Speaker: "乙", Text: "第二条", StartMillis: 3000, EndMillis: 6000, Position: 2},
		},
	})
	if err != nil {
		stored, getErr := st.GetProject(ctx, project.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		t.Fatalf("第二个显式 ID 复用了首次调用已关闭的所有者查询资源：err=%v revision=%d", err, stored.Revision)
	}
	if result.Count != 2 || result.Project.Revision != project.Revision+1 {
		t.Fatalf("显式 ID 批次结果不完整：%+v", result)
	}
}
