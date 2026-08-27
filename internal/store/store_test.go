package store

import (
	"context"
	"errors"
	"path/filepath"
	"stagecaption/internal/domain"
	"testing"
	"time"
)

func testProject(t *testing.T, s *Store) domain.CaptionProject {
	t.Helper()
	p, err := domain.NewProject("p1", "测试演出", "v1", 25, 10000, "开场", "编辑", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateProject(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRevisionConflictAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := testProject(t, s)
	ctx := context.Background()
	lease := Lease{ProjectID: p.ID, Scene: "一", Token: "token-a", Holder: "编辑", ExpiresAt: time.Now().Add(time.Minute)}
	if _, err = s.AcquireLease(ctx, lease); err != nil {
		t.Fatal(err)
	}
	updated, err := s.Write(ctx, p.ID, p.Revision, "编辑", "cue", false, func(tx *Tx, current *domain.CaptionProject) (string, error) {
		if err := tx.RequireLease(ctx, "一", "token-a", "编辑"); err != nil {
			return "", err
		}
		current.Status = domain.StatusValidation
		return "测试修订", tx.UpsertCue(ctx, domain.CaptionCue{ID: "c1", ProjectID: p.ID, Scene: "一", Text: "字幕", StartMillis: 0, EndMillis: 3000, Position: 1, UpdatedBy: "编辑"})
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Write(ctx, p.ID, p.Revision, "编辑", "stale", false, func(*Tx, *domain.CaptionProject) (string, error) { return "", nil })
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) || conflict.CurrentRevision != updated.Revision {
		t.Fatalf("未返回当前修订：%v", err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	restored, err := s.GetProject(ctx, p.ID)
	if err != nil || restored.Revision != 2 || restored.Status != domain.StatusValidation {
		t.Fatalf("重启恢复失败：%+v %v", restored, err)
	}
	snap, err := s.GetSnapshot(ctx, p.ID, 2)
	if err != nil || len(snap.Cues) != 1 {
		t.Fatalf("修订快照失败：%v", err)
	}
}

func TestLeaseCompetitionAndBarrier(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "lease.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	ctx := context.Background()
	_, err = s.AcquireLease(ctx, Lease{ProjectID: p.ID, Scene: "一", Token: "a", Holder: "甲", ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AcquireLease(ctx, Lease{ProjectID: p.ID, Scene: "一", Token: "b", Holder: "乙", ExpiresAt: time.Now().Add(time.Minute)})
	if !errors.Is(err, domain.ErrLeaseRequired) {
		t.Fatalf("竞争租约应失败：%v", err)
	}
	if err = s.SetBarrier(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	_, err = s.AcquireLease(ctx, Lease{ProjectID: p.ID, Scene: "二", Token: "c", Holder: "丙"})
	if !errors.Is(err, domain.ErrWriteBarrier) {
		t.Fatalf("写屏障应阻止租约：%v", err)
	}
}
