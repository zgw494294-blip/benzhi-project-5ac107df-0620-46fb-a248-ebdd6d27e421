package domain

import (
	"fmt"
	"strconv"
	"strings"
)

type BatchCueRow struct {
	Line        int    `json:"line"`
	ID          string `json:"id,omitempty"`
	Speaker     string `json:"speaker"`
	Text        string `json:"text"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Position    int    `json:"position"`
}

type BatchRowError struct {
	Line    int    `json:"line"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type BatchValidationError struct {
	Rows []BatchRowError `json:"rows"`
}

func (e *BatchValidationError) Error() string {
	return fmt.Sprintf("批量字幕有 %d 行未通过校验", len(e.Rows))
}

// ParseBatchCueText 接受制表符分隔的“说话人、正文、开始、结束、位置”，
// 也接受第一列为字幕 ID 的六列形式以更新既有字幕。
func ParseBatchCueText(text string) ([]BatchCueRow, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	rows := make([]BatchCueRow, 0, len(lines))
	var failures []BatchRowError
	for index, raw := range lines {
		line := index + 1
		if strings.TrimSpace(raw) == "" {
			continue
		}
		columns := strings.Split(raw, "\t")
		if len(columns) != 5 && len(columns) != 6 {
			failures = append(failures, BatchRowError{Line: line, Field: "columns", Message: "每行须为 5 列，更新既有字幕时可在首列增加字幕 ID"})
			continue
		}
		offset := 0
		row := BatchCueRow{Line: line}
		if len(columns) == 6 {
			row.ID, offset = strings.TrimSpace(columns[0]), 1
		}
		row.Speaker = columns[offset]
		row.Text = columns[offset+1]
		start, errStart := ParseMillis(columns[offset+2])
		end, errEnd := ParseMillis(columns[offset+3])
		position, errPosition := strconv.Atoi(strings.TrimSpace(columns[offset+4]))
		if errStart != nil {
			failures = append(failures, BatchRowError{Line: line, Field: "start", Message: "开始时间码无效"})
		}
		if errEnd != nil {
			failures = append(failures, BatchRowError{Line: line, Field: "end", Message: "结束时间码无效"})
		}
		if errPosition != nil {
			failures = append(failures, BatchRowError{Line: line, Field: "position", Message: "位置必须是整数"})
		}
		if errStart == nil && errEnd == nil && errPosition == nil {
			row.StartMillis, row.EndMillis, row.Position = start, end, position
			rows = append(rows, row)
		}
	}
	if len(failures) > 0 {
		return nil, &BatchValidationError{Rows: failures}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w：批量字幕不能为空", ErrValidation)
	}
	return rows, nil
}

func ParseMillis(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("空时间码")
	}
	if !strings.Contains(value, ":") {
		return strconv.ParseInt(value, 10, 64)
	}
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("时间码格式错误")
	}
	hours, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || minutes < 0 || minutes > 59 {
		return 0, fmt.Errorf("分钟无效")
	}
	secondParts := strings.Split(parts[2], ".")
	if len(secondParts) != 2 || len(secondParts[1]) != 3 {
		return 0, fmt.Errorf("毫秒须为三位")
	}
	seconds, err := strconv.ParseInt(secondParts[0], 10, 64)
	if err != nil || seconds < 0 || seconds > 59 {
		return 0, fmt.Errorf("秒无效")
	}
	millis, err := strconv.ParseInt(secondParts[1], 10, 64)
	if err != nil {
		return 0, err
	}
	if hours < 0 {
		return 0, fmt.Errorf("小时无效")
	}
	return hours*3600000 + minutes*60000 + seconds*1000 + millis, nil
}
