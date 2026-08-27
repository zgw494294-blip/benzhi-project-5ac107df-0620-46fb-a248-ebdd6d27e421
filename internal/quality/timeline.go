package quality

import (
	"fmt"
	"stagecaption/internal/domain"
	"time"
)

func (e *Engine) checkTimeline(p domain.CaptionProject, cues []domain.CaptionCue, now time.Time) []domain.QualityFinding {
	var out []domain.QualityFinding
	if len(cues) == 0 {
		return append(out, finding(p.ID, "", "EMPTY", domain.SeverityBlocking, "时间轴没有字幕", "0", now))
	}
	previousEnd := int64(0)
	for i, c := range cues {
		stay := c.EndMillis - c.StartMillis
		if stay < e.MinStayMillis {
			out = append(out, finding(p.ID, c.ID, "MIN_STAY", domain.SeverityBlocking, "字幕停留时间短于最低要求", durationLabel(stay), now))
		}
		if i > 0 && c.StartMillis < cues[i-1].EndMillis {
			overlap := cues[i-1].EndMillis - c.StartMillis
			out = append(out, finding(p.ID, c.ID, "OVERLAP", domain.SeverityBlocking, "字幕与上一条时间重叠", durationLabel(overlap), now))
		}
		if c.StartMillis-previousEnd > e.GapWarningMillis {
			out = append(out, finding(p.ID, c.ID, "COVERAGE_GAP", domain.SeverityWarning, "字幕覆盖存在较长空档", fmt.Sprintf("%d-%d", previousEnd, c.StartMillis), now))
		}
		if c.EndMillis > previousEnd {
			previousEnd = c.EndMillis
		}
	}
	if p.DurationMillis-previousEnd > e.GapWarningMillis {
		out = append(out, finding(p.ID, "", "COVERAGE_GAP", domain.SeverityWarning, "演出结尾存在较长字幕空档", fmt.Sprintf("%d-%d", previousEnd, p.DurationMillis), now))
	}
	return out
}
