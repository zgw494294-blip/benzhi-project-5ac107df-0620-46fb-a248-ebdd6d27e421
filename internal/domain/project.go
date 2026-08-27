package domain

import (
	"fmt"
	"strings"
	"time"
)

type CaptionProject struct {
	ID                string        `json:"id"`
	Title             string        `json:"title"`
	ProductionVersion string        `json:"productionVersion"`
	FrameRate         float64       `json:"frameRate"`
	DurationMillis    int64         `json:"durationMillis"`
	TimeOrigin        string        `json:"timeOrigin"`
	Status            ProjectStatus `json:"status"`
	Revision          int64         `json:"revision"`
	LastEditor        string        `json:"lastEditor"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`
}

func NewProject(id, title, version string, frameRate float64, duration int64, origin, actor string, now time.Time) (CaptionProject, error) {
	p := CaptionProject{ID: strings.TrimSpace(id), Title: strings.TrimSpace(title), ProductionVersion: strings.TrimSpace(version), FrameRate: frameRate, DurationMillis: duration, TimeOrigin: strings.TrimSpace(origin), Status: StatusDraft, Revision: 1, LastEditor: strings.TrimSpace(actor), CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	if err := p.Validate(); err != nil {
		return CaptionProject{}, err
	}
	return p, nil
}

func (p CaptionProject) Validate() error {
	if p.ID == "" || len(p.ID) > 80 {
		return fmt.Errorf("%w：项目 ID 不合法", ErrValidation)
	}
	if p.Title == "" || len([]rune(p.Title)) > 120 {
		return fmt.Errorf("%w：演出标题为 1 至 120 字", ErrValidation)
	}
	if p.ProductionVersion == "" || len([]rune(p.ProductionVersion)) > 60 {
		return fmt.Errorf("%w：版本为 1 至 60 字", ErrValidation)
	}
	if p.FrameRate <= 0 || p.FrameRate > 120 {
		return fmt.Errorf("%w：帧率必须大于 0 且不超过 120", ErrValidation)
	}
	if p.DurationMillis < 1000 || p.DurationMillis > 24*60*60*1000 {
		return fmt.Errorf("%w：总时长须在 1 秒至 24 小时之间", ErrValidation)
	}
	if p.TimeOrigin == "" || len([]rune(p.TimeOrigin)) > 80 {
		return fmt.Errorf("%w：台本时间基准不能为空", ErrValidation)
	}
	if !p.Status.Valid() {
		return fmt.Errorf("%w：未知项目状态", ErrValidation)
	}
	return nil
}

func (p CaptionProject) Editable() error {
	if p.Status == StatusLocked {
		return ErrLocked
	}
	return nil
}
