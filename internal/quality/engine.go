package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"stagecaption/internal/domain"
)

type Engine struct {
	Now               func() time.Time
	MinStayMillis     int64
	MaxCharsPerSecond float64
	GapWarningMillis  int64
}

func New() *Engine {
	return &Engine{Now: time.Now, MinStayMillis: 800, MaxCharsPerSecond: 22, GapWarningMillis: 5000}
}

func (e *Engine) Check(project domain.CaptionProject, cues []domain.CaptionCue) []domain.QualityFinding {
	ordered := append([]domain.CaptionCue(nil), cues...)
	domain.SortCues(ordered)
	now := e.Now().UTC()
	var out []domain.QualityFinding
	out = append(out, e.checkTimeline(project, ordered, now)...)
	out = append(out, e.checkText(project, ordered, now)...)
	domain.SortFindings(out, ordered)
	return out
}

func finding(projectID, cueID, code string, severity domain.Severity, message, value string, now time.Time) domain.QualityFinding {
	h := sha256.Sum256([]byte(projectID + "\x00" + cueID + "\x00" + code + "\x00" + value))
	return domain.QualityFinding{ID: "qf_" + hex.EncodeToString(h[:8]), ProjectID: projectID, CueID: cueID, RuleCode: code, Severity: severity, Message: message, ObservedValue: value, CreatedAt: now}
}

func durationLabel(ms int64) string { return fmt.Sprintf("%dms", ms) }
