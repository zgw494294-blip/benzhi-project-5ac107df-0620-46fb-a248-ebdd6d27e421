package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func CanonicalSnapshot(project CaptionProject, cues []CaptionCue) ([]byte, error) {
	copyCues := append([]CaptionCue(nil), cues...)
	SortCues(copyCues)
	v := struct {
		ID                string       `json:"id"`
		Title             string       `json:"title"`
		ProductionVersion string       `json:"productionVersion"`
		FrameRate         float64      `json:"frameRate"`
		DurationMillis    int64        `json:"durationMillis"`
		TimeOrigin        string       `json:"timeOrigin"`
		Revision          int64        `json:"revision"`
		Cues              []CaptionCue `json:"cues"`
	}{project.ID, project.Title, project.ProductionVersion, project.FrameRate, project.DurationMillis, project.TimeOrigin, project.Revision, copyCues}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("编码规范快照：%w", err)
	}
	return bytes.TrimSpace(b.Bytes()), nil
}
