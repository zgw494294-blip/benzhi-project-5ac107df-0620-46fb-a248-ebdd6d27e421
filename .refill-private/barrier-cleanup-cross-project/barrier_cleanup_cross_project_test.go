package barriercleanupcrossproject_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"stagecaption/internal/domain"
	"stagecaption/internal/quality"
	"stagecaption/internal/service"
	"stagecaption/internal/store"
)

func prepareReviewProject(t *testing.T, svc *service.Service, title string) domain.CaptionProject {
	t.Helper()
	ctx := context.Background()
	project, err := svc.CreateProject(ctx, service.CreateProjectInput{
		Title: title, ProductionVersion: "v1", FrameRate: 25,
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
	return project
}

func TestConcurrentReviewsMustReleaseOwnBarriers(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "barrier-owner.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	engine := quality.New()
	svc := service.New(st, engine)
	projectA := prepareReviewProject(t, svc, "复核项目 A")
	projectB := prepareReviewProject(t, svc, "复核项目 B")

	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	var calls atomic.Int32
	engine.Now = func() time.Time {
		switch calls.Add(1) {
		case 1:
			close(firstEntered)
			<-releaseFirst
		case 2:
			close(secondEntered)
			<-releaseSecond
		}
		return time.Unix(100, 0).UTC()
	}

	type reviewResult struct{ err error }
	firstDone := make(chan reviewResult, 1)
	secondDone := make(chan reviewResult, 1)
	go func() {
		_, _, reviewErr := svc.Review(context.Background(), projectA.ID, service.ReviewInput{
			ExpectedRevision: projectA.Revision, Reviewer: "记录员", Decision: "lock",
		})
		firstDone <- reviewResult{err: reviewErr}
	}()
	<-firstEntered
	go func() {
		_, _, reviewErr := svc.Review(context.Background(), projectB.ID, service.ReviewInput{
			ExpectedRevision: projectB.Revision, Reviewer: "独立复核员", Decision: "lock",
		})
		secondDone <- reviewResult{err: reviewErr}
	}()
	<-secondEntered

	close(releaseFirst)
	first := <-firstDone
	_, _, leaseErr := svc.AcquireLease(context.Background(), projectA.ID, service.LeaseInput{
		Scene: "第二场", Actor: "编辑乙",
	})
	close(releaseSecond)
	second := <-secondDone

	// 让两个屏障写入在同一受控起点并发执行，确保 -race 也能直接观察共享所有权字段。
	startWrites := make(chan struct{})
	var writes sync.WaitGroup
	writes.Add(2)
	go func() {
		defer writes.Done()
		<-startWrites
		_ = st.SetBarrier(context.Background(), projectA.ID, true)
	}()
	go func() {
		defer writes.Done()
		<-startWrites
		_ = st.SetBarrier(context.Background(), projectB.ID, true)
	}()
	close(startWrites)
	writes.Wait()

	if !errors.Is(first.err, domain.ErrReviewerEditor) {
		t.Fatalf("项目 A 应因复核员不独立而失败：%v", first.err)
	}
	if second.err != nil {
		t.Fatalf("项目 B 的独立复核应完成：%v", second.err)
	}
	if errors.Is(leaseErr, domain.ErrWriteBarrier) {
		t.Fatalf("项目 A 的失败复核清理了项目 B 的屏障，却把自身屏障永久遗留")
	}
	if leaseErr != nil {
		t.Fatalf("项目 A 复核退出后取得租约失败：%v", leaseErr)
	}
}
