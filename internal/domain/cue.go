package domain

import (
	"fmt"
	"sort"
	"strings"
)

type CaptionCue struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Scene       string `json:"scene"`
	Speaker     string `json:"speaker"`
	Text        string `json:"text"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Position    int    `json:"position"`
	Revision    int64  `json:"revision"`
	UpdatedBy   string `json:"updatedBy"`
}

func NormalizeCue(c CaptionCue, duration int64) (CaptionCue, error) {
	c.ID = strings.TrimSpace(c.ID)
	c.ProjectID = strings.TrimSpace(c.ProjectID)
	c.Scene = strings.TrimSpace(c.Scene)
	c.Speaker = strings.TrimSpace(c.Speaker)
	c.Text = strings.TrimSpace(strings.ReplaceAll(c.Text, "\r\n", "\n"))
	c.UpdatedBy = strings.TrimSpace(c.UpdatedBy)
	if c.ID == "" || c.ProjectID == "" {
		return c, fmt.Errorf("%w：字幕和项目 ID 不能为空", ErrValidation)
	}
	if c.Scene == "" || len([]rune(c.Scene)) > 80 {
		return c, fmt.Errorf("%w：场次为 1 至 80 字", ErrValidation)
	}
	if len([]rune(c.Speaker)) > 80 {
		return c, fmt.Errorf("%w：说话人不能超过 80 字", ErrValidation)
	}
	if c.Text == "" || len([]rune(c.Text)) > 500 {
		return c, fmt.Errorf("%w：字幕正文为 1 至 500 字", ErrValidation)
	}
	if c.StartMillis < 0 || c.EndMillis <= c.StartMillis || c.EndMillis > duration {
		return c, fmt.Errorf("%w：字幕时间范围不合法", ErrValidation)
	}
	if c.Position < 0 {
		c.Position = 0
	}
	if c.UpdatedBy == "" {
		return c, fmt.Errorf("%w：编辑者不能为空", ErrValidation)
	}
	return c, nil
}

func SortCues(cues []CaptionCue) {
	sort.SliceStable(cues, func(i, j int) bool {
		if cues[i].StartMillis != cues[j].StartMillis {
			return cues[i].StartMillis < cues[j].StartMillis
		}
		if cues[i].Scene != cues[j].Scene {
			return cues[i].Scene < cues[j].Scene
		}
		if cues[i].Position != cues[j].Position {
			return cues[i].Position < cues[j].Position
		}
		return cues[i].ID < cues[j].ID
	})
	for i := range cues {
		cues[i].Position = i + 1
	}
}

func FormatTimestamp(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	milli := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, milli)
}
