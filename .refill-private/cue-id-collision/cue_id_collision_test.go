package cueidcollision_test

import (
	"context"
	"path/filepath"
	"testing"

	"stagecaption/internal/quality"
	"stagecaption/internal/service"
	"stagecaption/internal/store"
)

func TestCrossProjectCueIDMustNotCommit(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "collision.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	app := service.New(st, quality.New())
	ctx := context.Background()

	first, err := app.CreateProject(ctx, service.CreateProjectInput{Title: "第一项目", ProductionVersion: "v1", FrameRate: 25, DurationMillis: 6000, TimeOrigin: "开场", Actor: "甲"})
	if err != nil {
		t.Fatal(err)
	}
	firstToken, _, err := app.AcquireLease(ctx, first.ID, service.LeaseInput{Scene: "第一场", Actor: "甲"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.UpsertCue(ctx, first.ID, service.CueInput{ID: "shared-cue", Scene: "第一场", Text: "第一项目字幕", StartMillis: 0, EndMillis: 6000, Position: 1, ExpectedRevision: first.Revision, Actor: "甲", LeaseToken: firstToken})
	if err != nil {
		t.Fatal(err)
	}

	second, err := app.CreateProject(ctx, service.CreateProjectInput{Title: "第二项目", ProductionVersion: "v1", FrameRate: 25, DurationMillis: 6000, TimeOrigin: "开场", Actor: "乙"})
	if err != nil {
		t.Fatal(err)
	}
	secondToken, _, err := app.AcquireLease(ctx, second.ID, service.LeaseInput{Scene: "第二场", Actor: "乙"})
	if err != nil {
		t.Fatal(err)
	}
	beforeRevision := second.Revision
	_, err = app.UpsertCue(ctx, second.ID, service.CueInput{ID: "shared-cue", Scene: "第二场", Text: "第二项目字幕", StartMillis: 0, EndMillis: 6000, Position: 1, ExpectedRevision: beforeRevision, Actor: "乙", LeaseToken: secondToken})
	if err == nil {
		after, getErr := st.GetProject(ctx, second.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		cues, listErr := st.ListCues(ctx, second.ID)
		if listErr != nil {
			t.Fatal(listErr)
		}
		t.Fatalf("跨项目字幕 ID 冲突被报告为成功：revision=%d，cues=%d；原修订=%d", after.Revision, len(cues), beforeRevision)
	}
}
