package service

import (
	"context"
	"errors"
	"path/filepath"
	"stagecaption/internal/domain"
	"stagecaption/internal/quality"
	"stagecaption/internal/store"
	"testing"
)

func TestFullWorkflowAndBundleTamperDetection(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "flow.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New(st, quality.New())
	ctx := context.Background()
	p, err := s.CreateProject(ctx, CreateProjectInput{Title: "演出", ProductionVersion: "v1", FrameRate: 25, DurationMillis: 6000, TimeOrigin: "开场", Actor: "编辑甲"})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := s.AcquireLease(ctx, p.ID, LeaseInput{Scene: "一", Actor: "编辑甲"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.UpsertCue(ctx, p.ID, CueInput{Scene: "一", Speaker: "旁白", Text: "欢迎观看", StartMillis: 0, EndMillis: 6000, Position: 1, ExpectedRevision: p.Revision, Actor: "编辑甲", LeaseToken: token})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReleaseLease(ctx, p.ID, "一", token); err != nil {
		t.Fatal(err)
	}
	p, findings, err := s.Validate(ctx, p.ID, ValidateInput{ExpectedRevision: p.Revision, Actor: "质检"})
	if err != nil || domain.HasBlocking(findings) {
		t.Fatalf("校验失败：%v %+v", err, findings)
	}
	ws, _ := s.GetWorkspace(ctx, p.ID)
	p, issues, err := s.RecordRehearsal(ctx, p.ID, RehearsalInput{ExpectedRevision: p.Revision, Recorder: "记录", Issues: []IssueInput{{CueID: ws.Cues[0].ID, Kind: "语义", Blocking: true, Note: "调整用词"}}})
	if err != nil {
		t.Fatal(err)
	}
	token, _, _ = s.AcquireLease(ctx, p.ID, LeaseInput{Scene: "一", Actor: "编辑乙"})
	p, _, err = s.Remediate(ctx, p.ID, RemediationInput{ExpectedRevision: p.Revision, Actor: "编辑乙", Scene: "一", LeaseToken: token, Cue: CaptionCuePatch{ID: ws.Cues[0].ID, Speaker: "旁白", Text: "欢迎欣赏", StartMillis: 0, EndMillis: 6000, Position: 1}, ResolvedIssueIDs: []string{issues[0].ID}, ResolutionNote: "已调整"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.StatusReview {
		t.Fatalf("状态=%s", p.Status)
	}
	if err = s.ReleaseLease(ctx, p.ID, "一", token); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.Review(ctx, p.ID, ReviewInput{ExpectedRevision: p.Revision, Reviewer: "编辑乙", Decision: "lock"})
	if !errors.Is(err, domain.ErrReviewerEditor) {
		t.Fatalf("最后编辑者锁版应失败：%v", err)
	}
	p, release, err := s.Review(ctx, p.ID, ReviewInput{ExpectedRevision: p.Revision, Reviewer: "独立复核", Decision: "lock", Note: "通过"})
	if err != nil || p.Status != domain.StatusLocked || release == nil {
		t.Fatalf("锁版失败：%v", err)
	}
	files, err := s.GetBundle(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyBundle(files).Valid {
		t.Fatal("原始播出包应有效")
	}
	files.WebVTT = append(files.WebVTT, 'x')
	if VerifyBundle(files).Valid {
		t.Fatal("篡改播出包不应通过")
	}
	_, err = s.UpsertCue(ctx, p.ID, CueInput{})
	if !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("锁版后编辑应失败：%v", err)
	}
}
