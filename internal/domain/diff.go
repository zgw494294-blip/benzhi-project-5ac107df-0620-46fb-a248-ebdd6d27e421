package domain

import "fmt"

type CueFieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

type CueDifference struct {
	CueID   string           `json:"cueId"`
	Kind    string           `json:"kind"`
	Before  *CaptionCue      `json:"before,omitempty"`
	After   *CaptionCue      `json:"after,omitempty"`
	Changes []CueFieldChange `json:"changes,omitempty"`
}

func CompareSnapshots(from, to RevisionSnapshot) ([]CueDifference, error) {
	if from.Project.ID == "" || from.Project.ID != to.Project.ID {
		return nil, fmt.Errorf("%w：修订快照不属于同一项目", ErrValidation)
	}
	old, current := map[string]CaptionCue{}, map[string]CaptionCue{}
	for _, cue := range from.Cues {
		old[cue.ID] = cue
	}
	for _, cue := range to.Cues {
		current[cue.ID] = cue
	}
	ids := make([]CaptionCue, 0, len(from.Cues)+len(to.Cues))
	seen := map[string]bool{}
	for _, cue := range append(append([]CaptionCue{}, from.Cues...), to.Cues...) {
		if !seen[cue.ID] {
			ids, seen[cue.ID] = append(ids, cue), true
		}
	}
	SortCues(ids)
	out := make([]CueDifference, 0)
	for _, key := range ids {
		a, aok := old[key.ID]
		b, bok := current[key.ID]
		if !aok {
			copy := b
			out = append(out, CueDifference{CueID: key.ID, Kind: "新增", After: &copy})
			continue
		}
		if !bok {
			copy := a
			out = append(out, CueDifference{CueID: key.ID, Kind: "删除", Before: &copy})
			continue
		}
		changes := make([]CueFieldChange, 0, 6)
		if a.Text != b.Text {
			changes = append(changes, CueFieldChange{"text", a.Text, b.Text})
		}
		if a.Speaker != b.Speaker {
			changes = append(changes, CueFieldChange{"speaker", a.Speaker, b.Speaker})
		}
		if a.Scene != b.Scene {
			changes = append(changes, CueFieldChange{"scene", a.Scene, b.Scene})
		}
		if a.StartMillis != b.StartMillis {
			changes = append(changes, CueFieldChange{"startMillis", a.StartMillis, b.StartMillis})
		}
		if a.EndMillis != b.EndMillis {
			changes = append(changes, CueFieldChange{"endMillis", a.EndMillis, b.EndMillis})
		}
		if a.Position != b.Position {
			changes = append(changes, CueFieldChange{"position", a.Position, b.Position})
		}
		if len(changes) > 0 {
			before, after := a, b
			out = append(out, CueDifference{CueID: key.ID, Kind: "修改", Before: &before, After: &after, Changes: changes})
		}
	}
	return out, nil
}
