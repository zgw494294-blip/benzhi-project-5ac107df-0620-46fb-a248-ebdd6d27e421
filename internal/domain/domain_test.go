package domain

import (
	"bytes"
	"testing"
	"time"
)

func TestStateMachineIsExplicit(t *testing.T) {
	cases := []struct {
		from, to ProjectStatus
		ok       bool
	}{{StatusDraft, StatusValidation, true}, {StatusValidation, StatusRehearsal, true}, {StatusRehearsal, StatusRemediation, true}, {StatusRehearsal, StatusReview, true}, {StatusRemediation, StatusReview, true}, {StatusReview, StatusLocked, true}, {StatusDraft, StatusLocked, false}, {StatusLocked, StatusDraft, false}}
	for _, tc := range cases {
		if got := CanTransition(tc.from, tc.to); got != tc.ok {
			t.Fatalf("%s -> %s = %v", tc.from, tc.to, got)
		}
	}
}

func TestCanonicalSnapshotIgnoresInputOrder(t *testing.T) {
	p, _ := NewProject("p", "演出", "v1", 25, 10000, "铃响", "甲", time.Unix(1, 0))
	a := []CaptionCue{{ID: "b", ProjectID: "p", Scene: "二", Text: "后", StartMillis: 3000, EndMillis: 5000, UpdatedBy: "甲"}, {ID: "a", ProjectID: "p", Scene: "一", Text: "前", StartMillis: 0, EndMillis: 2000, UpdatedBy: "甲"}}
	b := []CaptionCue{a[1], a[0]}
	x, err := CanonicalSnapshot(p, a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := CanonicalSnapshot(p, b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(x, y) {
		t.Fatalf("规范快照随输入顺序变化")
	}
}

func TestNormalizeCueRejectsInvalidRange(t *testing.T) {
	_, err := NormalizeCue(CaptionCue{ID: "c", ProjectID: "p", Scene: "一", Text: "字幕", StartMillis: 1000, EndMillis: 1000, UpdatedBy: "甲"}, 5000)
	if err == nil {
		t.Fatal("应拒绝零时长字幕")
	}
	if got := FormatTimestamp(3723004); got != "01:02:03.004" {
		t.Fatalf("时间码=%s", got)
	}
}
