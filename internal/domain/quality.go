package domain

import (
	"sort"
	"time"
)

type Severity string

const (
	SeverityBlocking Severity = "blocking"
	SeverityWarning  Severity = "warning"
)

type QualityFinding struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"projectId"`
	CueID         string    `json:"cueId,omitempty"`
	RuleCode      string    `json:"ruleCode"`
	Severity      Severity  `json:"severity"`
	Message       string    `json:"message"`
	ObservedValue string    `json:"observedValue"`
	CreatedAt     time.Time `json:"createdAt"`
}

func HasBlocking(findings []QualityFinding) bool {
	for _, f := range findings {
		if f.Severity == SeverityBlocking {
			return true
		}
	}
	return false
}

var QualityRuleOrder = []string{"EMPTY", "COVERAGE_GAP", "OVERLAP", "MIN_STAY", "READING_SPEED", "LINE_COUNT", "LINE_LENGTH", "FORBIDDEN_STYLE"}

func QualityRuleRank(code string) int {
	for i, candidate := range QualityRuleOrder {
		if code == candidate {
			return i
		}
	}
	return len(QualityRuleOrder)
}

func SortFindings(findings []QualityFinding, cues []CaptionCue) {
	positions := make(map[string]int, len(cues))
	scenes := make(map[string]string, len(cues))
	for i, cue := range cues {
		positions[cue.ID], scenes[cue.ID] = i, cue.Scene
	}
	sort.SliceStable(findings, func(i, j int) bool {
		ri, rj := QualityRuleRank(findings[i].RuleCode), QualityRuleRank(findings[j].RuleCode)
		if ri != rj {
			return ri < rj
		}
		if findings[i].RuleCode != findings[j].RuleCode {
			return findings[i].RuleCode < findings[j].RuleCode
		}
		if scenes[findings[i].CueID] != scenes[findings[j].CueID] {
			return scenes[findings[i].CueID] < scenes[findings[j].CueID]
		}
		if positions[findings[i].CueID] != positions[findings[j].CueID] {
			return positions[findings[i].CueID] < positions[findings[j].CueID]
		}
		return findings[i].ID < findings[j].ID
	})
}
