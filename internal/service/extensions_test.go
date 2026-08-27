package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"stagecaption/internal/domain"
	"stagecaption/internal/quality"
	"stagecaption/internal/store"
)

func newExtensionService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "extensions.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st, quality.New()), context.Background()
}

func TestBatchCueQueueIsAtomicAndFilterable(t *testing.T) {
	s, ctx := newExtensionService(t)
	p, err := s.CreateProject(ctx, CreateProjectInput{Title: "巡演字幕", ProductionVersion: "夏季版", FrameRate: 25, DurationMillis: 9000, TimeOrigin: "开场", Actor: "编辑"})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := s.AcquireLease(ctx, p.ID, LeaseInput{Scene: "第一场", Actor: "编辑"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.BatchUpsertCues(ctx, p.ID, BatchCueInput{Scene: "第一场", Actor: "编辑", LeaseToken: token, ExpectedRevision: p.Revision, Paste: "旁白\t第一条\t00:00:00.000\t00:00:03.000\t1\n甲\t第二条\t3000\t6000\t2\n乙\t第三条\t6000\t9000\t3"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 3 || result.Project.Revision != p.Revision+1 {
		t.Fatalf("批量结果=%+v", result)
	}
	queue, err := s.QueryProjects(ctx, ProjectQueueFilter{Keyword: "夏季", Status: string(domain.StatusValidation), Sort: "updatedAt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].CueCount != 3 {
		t.Fatalf("队列摘要=%+v", queue)
	}
	before := result.Project
	_, err = s.BatchUpsertCues(ctx, p.ID, BatchCueInput{Scene: "第一场", Actor: "编辑", LeaseToken: token, ExpectedRevision: before.Revision, Paste: "旁白\t合法行\t0\t1000\t1\n旁白\t越界行\t8000\t10000\t2"})
	var rowErr *domain.BatchValidationError
	if !errors.As(err, &rowErr) || len(rowErr.Rows) != 1 {
		t.Fatalf("应返回逐行错误：%v", err)
	}
	after, _ := s.Store.GetProject(ctx, p.ID)
	cues, _ := s.Store.ListCues(ctx, p.ID)
	if after.Revision != before.Revision || len(cues) != 3 {
		t.Fatalf("失败批次发生部分写入：r%d cues=%d", after.Revision, len(cues))
	}
	if _, err = s.QueryProjects(ctx, ProjectQueueFilter{Keyword: "   "}); err == nil {
		t.Fatal("空白关键词应被拒绝")
	}
	if _, err = s.QueryProjects(ctx, ProjectQueueFilter{Status: "未知"}); err == nil {
		t.Fatal("未知状态应被拒绝")
	}
}

func TestIssueViewsBatchRemediationGateAndUploadedBundle(t *testing.T) {
	s, ctx := newExtensionService(t)
	p, err := s.CreateProject(ctx, CreateProjectInput{Title: "联合复演", ProductionVersion: "首演版", FrameRate: 25, DurationMillis: 9000, TimeOrigin: "铃声", Actor: "编辑甲"})
	if err != nil {
		t.Fatal(err)
	}
	token, _, _ := s.AcquireLease(ctx, p.ID, LeaseInput{Scene: "第一场", Actor: "编辑甲"})
	longText := strings.Repeat("可", 50)
	batch, err := s.BatchUpsertCues(ctx, p.ID, BatchCueInput{Scene: "第一场", Actor: "编辑甲", LeaseToken: token, ExpectedRevision: p.Revision, Rows: []domain.BatchCueRow{{Speaker: "甲", Text: "第一条", StartMillis: 0, EndMillis: 3000, Position: 1}, {Speaker: "乙", Text: "第二条", StartMillis: 3000, EndMillis: 6000, Position: 2}, {Speaker: "旁白", Text: longText, StartMillis: 6000, EndMillis: 9000, Position: 3}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReleaseLease(ctx, p.ID, "第一场", token); err != nil {
		t.Fatal(err)
	}
	p, findings, err := s.Validate(ctx, p.ID, ValidateInput{ExpectedRevision: batch.Project.Revision, Actor: "质检"})
	if err != nil {
		t.Fatal(err)
	}
	if domain.HasBlocking(findings) {
		t.Fatalf("不应有阻断：%+v", findings)
	}
	findingView, err := s.QueryFindings(ctx, p.ID, FindingFilter{Severity: "warning", Rule: "READING_SPEED", Scene: "第一场"})
	if err != nil || findingView.Summary.Matched != 1 {
		t.Fatalf("质量筛选=%+v err=%v", findingView, err)
	}
	cues, _ := s.Store.ListCues(ctx, p.ID)
	p, issues, err := s.RecordRehearsal(ctx, p.ID, RehearsalInput{ExpectedRevision: p.Revision, Recorder: "记录员", Issues: []IssueInput{{CueID: cues[0].ID, Kind: "提前", Blocking: true, Note: "提前"}, {CueID: cues[1].ID, Kind: "滞后", Blocking: true, Note: "滞后"}, {CueID: cues[2].ID, Kind: "语义", Blocking: false, Note: "观察"}}})
	if err != nil {
		t.Fatal(err)
	}
	issueView, err := s.QueryIssues(ctx, p.ID, IssueFilter{Scene: "第一场"})
	if err != nil {
		t.Fatal(err)
	}
	if issueView.Summary.Pending != 2 || issueView.Summary.Observations != 1 || len(issueView.ReplayWindows) != 1 || len(issueView.ReplayWindows[0].IssueIDs) != 2 {
		t.Fatalf("问题视图=%+v", issueView)
	}
	token, _, err = s.AcquireLease(ctx, p.ID, LeaseInput{Scene: "第一场", Actor: "编辑乙"})
	if err != nil {
		t.Fatal(err)
	}
	beforeRevision := p.Revision
	remediation, err := s.BatchRemediate(ctx, p.ID, BatchRemediationInput{ExpectedRevision: p.Revision, Actor: "编辑乙", Scene: "第一场", LeaseToken: token, Patches: []RemediationPatch{{Cue: CaptionCuePatch{ID: cues[0].ID, Speaker: cues[0].Speaker, Text: "第一条已校正", StartMillis: 0, EndMillis: 3000, Position: 1}, ResolvedIssueIDs: []string{issues[0].ID}, ResolutionNote: "完成"}, {Cue: CaptionCuePatch{ID: cues[1].ID, Speaker: cues[1].Speaker, Text: "第二条已校正", StartMillis: 3000, EndMillis: 6000, Position: 2}, ResolvedIssueIDs: []string{issues[1].ID}, ResolutionNote: "完成"}}})
	if err != nil {
		t.Fatal(err)
	}
	if remediation.Project.Revision != beforeRevision+1 || remediation.Project.Status != domain.StatusReview {
		t.Fatalf("整改应只增加一个修订：%+v", remediation.Project)
	}
	comparison, err := s.DefaultComparison(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if comparison.FromRevision != beforeRevision || len(comparison.Differences) != 2 {
		t.Fatalf("默认差异=%+v", comparison)
	}
	gate, err := s.CheckLockGate(ctx, p.ID, "编辑乙")
	if err != nil {
		t.Fatal(err)
	}
	failed := map[string]bool{}
	for _, item := range gate.Items {
		if !item.Passed {
			failed[item.Code] = true
		}
	}
	if !failed["independent_reviewer"] || !failed["leases_clear"] {
		t.Fatalf("门禁未同时报告原因：%+v", gate)
	}
	if err = s.ReleaseLease(ctx, p.ID, "第一场", token); err != nil {
		t.Fatal(err)
	}
	gate, err = s.CheckLockGate(ctx, p.ID, "独立复核")
	if err != nil || !gate.Passed {
		t.Fatalf("门禁应通过：%+v %v", gate, err)
	}
	p, release, err := s.Review(ctx, p.ID, ReviewInput{ExpectedRevision: remediation.Project.Revision, Reviewer: "独立复核", Decision: "lock", Note: "通过"})
	if err != nil || release == nil {
		t.Fatalf("锁版失败：%v", err)
	}
	files, err := s.GetBundle(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := s.VerifyUploadedBundle(ctx, p.ID, UploadedBundle{WebVTT: files.WebVTT, Manifest: files.Manifest, Credential: files.Credential})
	if err != nil || !verified.Valid {
		t.Fatalf("原样回读失败：%+v %v", verified, err)
	}
	tampered := append([]byte(nil), files.WebVTT...)
	tampered[len(tampered)-2] = 'X'
	verified, err = s.VerifyUploadedBundle(ctx, p.ID, UploadedBundle{WebVTT: tampered, Manifest: files.Manifest, Credential: files.Credential})
	if err != nil || verified.Valid || verified.WebVTT.Status != "摘要不符" || verified.Manifest.Status != "通过" || verified.Credential.Status != "通过" {
		t.Fatalf("篡改结论不精确：%+v %v", verified, err)
	}
	after, _ := s.Store.GetProject(ctx, p.ID)
	if after.Revision != p.Revision || after.Status != domain.StatusLocked {
		t.Fatal("回读验真不应修改项目")
	}
}
