package quality

import "stagecaption/internal/domain"

// CheckTargeted 返回完整规则结果，并标记与整改字幕相邻时间窗有关的发现。
// 保留完整结果可防止目标复验在局部修订后掩盖新引入的全局问题。
func (e *Engine) CheckTargeted(project domain.CaptionProject, cues []domain.CaptionCue, affectedIDs []string) ([]domain.QualityFinding, []domain.QualityFinding) {
	all := e.Check(project, cues)
	wanted := map[string]bool{}
	for _, id := range affectedIDs {
		wanted[id] = true
	}
	for _, c := range cues {
		if wanted[c.ID] {
			for _, neighbor := range cues {
				if neighbor.StartMillis <= c.EndMillis+2000 && neighbor.EndMillis >= c.StartMillis-2000 {
					wanted[neighbor.ID] = true
				}
			}
		}
	}
	targeted := make([]domain.QualityFinding, 0)
	for _, f := range all {
		if f.CueID == "" || wanted[f.CueID] {
			targeted = append(targeted, f)
		}
	}
	return all, targeted
}
