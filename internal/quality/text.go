package quality

import (
	"fmt"
	"regexp"
	"stagecaption/internal/domain"
	"strings"
	"time"
	"unicode/utf8"
)

var forbiddenStyle = regexp.MustCompile(`(?i)<\/?(?:font|span|style|script|b|i|u)(?:\s[^>]*)?>|\{\\[^}]+\}`)

func (e *Engine) checkText(p domain.CaptionProject, cues []domain.CaptionCue, now time.Time) []domain.QualityFinding {
	var out []domain.QualityFinding
	for _, c := range cues {
		plain := strings.ReplaceAll(c.Text, "\n", "")
		seconds := float64(c.EndMillis-c.StartMillis) / 1000
		cps := float64(utf8.RuneCountInString(plain)) / seconds
		if cps > e.MaxCharsPerSecond {
			out = append(out, finding(p.ID, c.ID, "READING_SPEED", domain.SeverityBlocking, "字幕阅读速度过快", fmt.Sprintf("%.1f 字/秒", cps), now))
		} else if cps > 15 {
			out = append(out, finding(p.ID, c.ID, "READING_SPEED", domain.SeverityWarning, "字幕阅读速度偏快", fmt.Sprintf("%.1f 字/秒", cps), now))
		}
		lines := strings.Split(c.Text, "\n")
		if len(lines) > 2 {
			out = append(out, finding(p.ID, c.ID, "LINE_COUNT", domain.SeverityBlocking, "字幕不得超过两行", fmt.Sprintf("%d 行", len(lines)), now))
		}
		for _, line := range lines {
			if utf8.RuneCountInString(line) > 22 {
				out = append(out, finding(p.ID, c.ID, "LINE_LENGTH", domain.SeverityWarning, "单行字幕超过 22 字", fmt.Sprintf("%d 字", utf8.RuneCountInString(line)), now))
				break
			}
		}
		if forbiddenStyle.MatchString(c.Text) {
			out = append(out, finding(p.ID, c.ID, "FORBIDDEN_STYLE", domain.SeverityBlocking, "字幕包含禁用样式或标签", "检测到样式标记", now))
		}
	}
	return out
}
