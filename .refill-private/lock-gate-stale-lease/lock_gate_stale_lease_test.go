package lockgatestalelease_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"stagecaption/internal/domain"
	"stagecaption/internal/quality"
	"stagecaption/internal/service"
	"stagecaption/internal/store"
)

func TestReviewMustRecheckLeasesAfterGateCache(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "stale-gate.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := service.New(st, quality.New())
	ctx := context.Background()

	project, err := svc.CreateProject(ctx, service.CreateProjectInput{
		Title: "租约门禁复现", ProductionVersion: "v1", FrameRate: 25,
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
		t.Fatalf("准备待排演项目失败：%v %+v", err, findings)
	}
	project, _, err = svc.RecordRehearsal(ctx, project.ID, service.RehearsalInput{ExpectedRevision: project.Revision, Recorder: "记录员"})
	if err != nil || project.Status != domain.StatusReview {
		t.Fatalf("准备待复核项目失败：%v %+v", err, project)
	}

	gate, err := svc.CheckLockGate(ctx, project.ID, "独立复核员")
	if err != nil || !gate.Passed {
		t.Fatalf("初始门禁应通过：%v %+v", err, gate)
	}
	activeToken, _, err := svc.AcquireLease(ctx, project.ID, service.LeaseInput{Scene: "第一场", Actor: "编辑乙"})
	if err != nil {
		t.Fatal(err)
	}

	locked, _, reviewErr := svc.Review(ctx, project.ID, service.ReviewInput{
		ExpectedRevision: project.Revision, Reviewer: "独立复核员", Decision: "lock",
	})
	var gateErr *service.LockGateError
	if !errors.As(reviewErr, &gateErr) || locked.Status == domain.StatusLocked {
		t.Fatalf("活动租约建立后仍复用旧门禁并锁版：err=%v status=%s token=%s", reviewErr, locked.Status, activeToken)
	}
}
