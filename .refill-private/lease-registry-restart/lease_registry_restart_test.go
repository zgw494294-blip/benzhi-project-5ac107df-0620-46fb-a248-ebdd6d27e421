package leaseregistryrestart_test

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

func TestPersistedLeaseMustRemainUsableAfterRestart(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "lease-restart.db")
	st, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(st, quality.New())
	ctx := context.Background()
	project, err := svc.CreateProject(ctx, service.CreateProjectInput{
		Title: "租约重启复现", ProductionVersion: "v1", FrameRate: 25,
		DurationMillis: 6000, TimeOrigin: "开场", Actor: "编辑甲",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := svc.AcquireLease(ctx, project.ID, service.LeaseInput{
		Scene: "第一场", Actor: "编辑甲", TTLSeconds: 900,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	leases, err := reopened.ListActiveLeases(ctx, project.ID)
	if err != nil || len(leases) != 1 || leases[0].Token != token {
		t.Fatalf("SQLite 中的有效租约未恢复：%v %+v", err, leases)
	}
	restartedService := service.New(reopened, quality.New())
	updated, err := restartedService.UpsertCue(ctx, project.ID, service.CueInput{
		Scene: "第一场", Speaker: "旁白", Text: "重启后继续编辑",
		StartMillis: 0, EndMillis: 6000, Position: 1,
		ExpectedRevision: project.Revision, Actor: "编辑甲", LeaseToken: token,
	})
	if errors.Is(err, domain.ErrLeaseRequired) {
		t.Fatalf("重启后数据库中的有效租约被空的进程内注册表拒绝")
	}
	if err != nil {
		t.Fatalf("重启后使用有效租约编辑失败：%v", err)
	}
	if updated.Revision != project.Revision+1 {
		t.Fatalf("重启后编辑未形成新修订：%+v", updated)
	}
}
