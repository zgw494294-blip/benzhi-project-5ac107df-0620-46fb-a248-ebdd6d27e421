package bundlecachealias_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"stagecaption/internal/domain"
	"stagecaption/internal/quality"
	"stagecaption/internal/service"
	"stagecaption/internal/store"
)

func TestBundleCacheMustIsolateReturnedBytes(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "bundle-cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := service.New(st, quality.New())
	ctx := context.Background()

	project, err := svc.CreateProject(ctx, service.CreateProjectInput{
		Title: "缓存隔离复现", ProductionVersion: "v1", FrameRate: 25,
		DurationMillis: 6000, TimeOrigin: "开场", Actor: "编辑甲",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := svc.AcquireLease(ctx, project.ID, service.LeaseInput{Scene: "第一场", Actor: "编辑甲"})
	if err != nil {
		t.Fatal(err)
	}
	project, err = svc.UpsertCue(ctx, project.ID, service.CueInput{
		Scene: "第一场", Speaker: "旁白", Text: "欢迎观看",
		StartMillis: 0, EndMillis: 6000, Position: 1,
		ExpectedRevision: project.Revision, Actor: "编辑甲", LeaseToken: token,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.ReleaseLease(ctx, project.ID, "第一场", token); err != nil {
		t.Fatal(err)
	}
	project, findings, err := svc.Validate(ctx, project.ID, service.ValidateInput{ExpectedRevision: project.Revision, Actor: "质检员"})
	if err != nil || domain.HasBlocking(findings) {
		t.Fatalf("准备锁版项目失败：%v %+v", err, findings)
	}
	project, _, err = svc.RecordRehearsal(ctx, project.ID, service.RehearsalInput{ExpectedRevision: project.Revision, Recorder: "记录员"})
	if err != nil {
		t.Fatal(err)
	}
	project, _, err = svc.Review(ctx, project.ID, service.ReviewInput{ExpectedRevision: project.Revision, Reviewer: "独立复核员", Decision: "lock"})
	if err != nil {
		t.Fatal(err)
	}

	first, err := svc.GetBundle(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte(nil), first.WebVTT...)
	first.WebVTT[0] = 'X'

	second, err := svc.GetBundle(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(second.WebVTT, want) || !service.VerifyBundle(second).Valid {
		t.Fatalf("第二次读取复用了调用方污染的 WebVTT 缓冲区")
	}
}
