package validationstaleretry_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"stagecaption/internal/domain"
	"stagecaption/internal/quality"
	"stagecaption/internal/service"
	"stagecaption/internal/store"
)

func TestValidationRetryMustRecomputeAfterConflict(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "validation-retry.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	engine := quality.New()
	svc := service.New(st, engine)
	ctx := context.Background()
	project, err := svc.CreateProject(ctx, service.CreateProjectInput{
		Title: "校验重试复现", ProductionVersion: "v1", FrameRate: 25,
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
		Scene: "第一场", Speaker: "旁白", Text: "完整覆盖",
		StartMillis: 0, EndMillis: 6000, Position: 1,
		ExpectedRevision: project.Revision, Actor: "编辑甲", LeaseToken: token,
	})
	if err != nil {
		t.Fatal(err)
	}
	cues, err := st.ListCues(ctx, project.ID)
	if err != nil || len(cues) != 1 {
		t.Fatalf("读取准备字幕失败：%v %+v", err, cues)
	}

	checkEntered := make(chan struct{})
	releaseCheck := make(chan struct{})
	var calls atomic.Int32
	engine.Now = func() time.Time {
		if calls.Add(1) == 1 {
			close(checkEntered)
			<-releaseCheck
		}
		return time.Unix(100, 0).UTC()
	}
	type validationResult struct {
		project  domain.CaptionProject
		findings []domain.QualityFinding
		err      error
	}
	validated := make(chan validationResult, 1)
	go func() {
		p, findings, validateErr := svc.Validate(ctx, project.ID, service.ValidateInput{
			ExpectedRevision: project.Revision, Actor: "质检员",
		})
		validated <- validationResult{project: p, findings: findings, err: validateErr}
	}()
	<-checkEntered

	concurrent, err := svc.UpsertCue(ctx, project.ID, service.CueInput{
		ID: cues[0].ID, Scene: "第一场", Speaker: "旁白", Text: "过短字幕",
		StartMillis: 0, EndMillis: 500, Position: 1,
		ExpectedRevision: project.Revision, Actor: "编辑甲", LeaseToken: token,
	})
	if err != nil {
		t.Fatal(err)
	}
	close(releaseCheck)
	result := <-validated

	stored, err := st.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	currentCues, err := st.ListCues(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	fresh := engine.Check(stored, currentCues)
	if result.err == nil && stored.Status == domain.StatusRehearsal && domain.HasBlocking(fresh) && !domain.HasBlocking(result.findings) {
		t.Fatalf("校验冲突重试复用了旧时间轴并把含阻断字幕的 r%d 推进到待排演", concurrent.Revision)
	}
	var conflict *domain.ConflictError
	if !errors.As(result.err, &conflict) {
		t.Fatalf("并发编辑后校验应返回修订冲突：%v %+v", result.err, result.project)
	}
}
