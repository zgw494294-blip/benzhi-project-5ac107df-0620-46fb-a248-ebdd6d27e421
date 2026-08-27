package quality

import (
	"reflect"
	"stagecaption/internal/domain"
	"testing"
	"time"
)

func TestCheckFindsTimelineAndTextProblemsDeterministically(t *testing.T) {
	engine := New()
	engine.Now = func() time.Time { return time.Unix(10, 0) }
	p := domain.CaptionProject{ID: "p", DurationMillis: 12000}
	cues := []domain.CaptionCue{{ID: "a", ProjectID: "p", Scene: "一", Text: "<b>这是一条非常非常非常非常非常非常非常非常非常非常长的字幕</b>", StartMillis: 6000, EndMillis: 6600}, {ID: "b", ProjectID: "p", Scene: "一", Text: "第二条", StartMillis: 6500, EndMillis: 9000}}
	first := engine.Check(p, cues)
	second := engine.Check(p, []domain.CaptionCue{cues[1], cues[0]})
	if !reflect.DeepEqual(first, second) {
		t.Fatal("规则结果必须稳定")
	}
	codes := map[string]bool{}
	for _, f := range first {
		codes[f.RuleCode] = true
	}
	for _, code := range []string{"COVERAGE_GAP", "MIN_STAY", "OVERLAP", "READING_SPEED", "FORBIDDEN_STYLE"} {
		if !codes[code] {
			t.Errorf("缺少规则 %s", code)
		}
	}
}

func TestTargetedKeepsGlobalConsistency(t *testing.T) {
	e := New()
	p := domain.CaptionProject{ID: "p", DurationMillis: 10000}
	cues := []domain.CaptionCue{{ID: "a", ProjectID: "p", Scene: "一", Text: "甲", StartMillis: 0, EndMillis: 3000}, {ID: "b", ProjectID: "p", Scene: "一", Text: "乙", StartMillis: 2500, EndMillis: 5000}, {ID: "c", ProjectID: "p", Scene: "二", Text: "丙", StartMillis: 7000, EndMillis: 10000}}
	all, targeted := e.CheckTargeted(p, cues, []string{"a"})
	if len(all) == 0 || len(targeted) == 0 {
		t.Fatal("应保留全量和目标结果")
	}
	if !domain.HasBlocking(all) {
		t.Fatal("应发现时间重叠")
	}
}
