package service

import (
	"context"
	"fmt"
	"strings"

	"stagecaption/internal/domain"
	"stagecaption/internal/store"
)

const maxBatchCues = 200

func (s *Service) BatchUpsertCues(ctx context.Context, projectID string, in BatchCueInput) (BatchCueResult, error) {
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return BatchCueResult{}, err
	}
	if p.Status != domain.StatusDraft && p.Status != domain.StatusValidation && p.Status != domain.StatusRemediation {
		return BatchCueResult{Project: p}, domain.ErrInvalidState
	}
	actor, scene := cleanActor(in.Actor), strings.TrimSpace(in.Scene)
	if actor == "" || scene == "" || len([]rune(scene)) > 80 {
		return BatchCueResult{Project: p}, fmt.Errorf("%w：场次和编辑者必须有效", domain.ErrValidation)
	}
	rows := append([]domain.BatchCueRow(nil), in.Rows...)
	if strings.TrimSpace(in.Paste) != "" {
		if len(rows) > 0 {
			return BatchCueResult{Project: p}, fmt.Errorf("%w：paste 与 rows 只能提交一种", domain.ErrValidation)
		}
		rows, err = domain.ParseBatchCueText(in.Paste)
		if err != nil {
			return BatchCueResult{Project: p}, err
		}
	}
	if len(rows) == 0 || len(rows) > maxBatchCues {
		return BatchCueResult{Project: p}, fmt.Errorf("%w：每批字幕须为 1 至 %d 条", domain.ErrValidation, maxBatchCues)
	}
	seen := map[string]int{}
	cues := make([]domain.CaptionCue, 0, len(rows))
	var failures []domain.BatchRowError
	for index, row := range rows {
		line := row.Line
		if line <= 0 {
			line = index + 1
		}
		id := strings.TrimSpace(row.ID)
		if id == "" {
			id = newID("cue_")
		} else if owner, ownerErr := s.Store.CueProject(ctx, id); ownerErr == nil && owner != projectID {
			failures = append(failures, domain.BatchRowError{Line: line, Field: "id", Message: "字幕标识属于其他项目"})
			continue
		} else if ownerErr != nil && ownerErr != domain.ErrNotFound {
			return BatchCueResult{Project: p}, ownerErr
		}
		if previous, ok := seen[id]; ok {
			failures = append(failures, domain.BatchRowError{Line: line, Field: "id", Message: fmt.Sprintf("字幕标识与第 %d 行重复", previous)})
			continue
		}
		seen[id] = line
		cue, e := domain.NormalizeCue(domain.CaptionCue{ID: id, ProjectID: projectID, Scene: scene, Speaker: row.Speaker, Text: row.Text, StartMillis: row.StartMillis, EndMillis: row.EndMillis, Position: row.Position, UpdatedBy: actor}, p.DurationMillis)
		if e != nil {
			failures = append(failures, domain.BatchRowError{Line: line, Field: "row", Message: e.Error()})
			continue
		}
		cues = append(cues, cue)
	}
	if len(failures) > 0 {
		return BatchCueResult{Project: p}, &domain.BatchValidationError{Rows: failures}
	}
	p, err = s.Store.Write(ctx, projectID, in.ExpectedRevision, actor, "batch_upsert_cues", false, func(tx *store.Tx, current *domain.CaptionProject) (string, error) {
		if err := tx.RequireLease(ctx, scene, in.LeaseToken, actor); err != nil {
			return "", err
		}
		if current.Status == domain.StatusDraft {
			if err := domain.RequireTransition(current.Status, domain.StatusValidation); err != nil {
				return "", err
			}
			current.Status = domain.StatusValidation
		}
		for _, cue := range cues {
			if err := tx.UpsertCue(ctx, cue); err != nil {
				return "", err
			}
		}
		return fmt.Sprintf("批量入轨成功 %d 条字幕", len(cues)), nil
	})
	if err != nil {
		return BatchCueResult{Project: p}, err
	}
	domain.SortCues(cues)
	return BatchCueResult{Project: p, Cues: cues, Count: len(cues)}, nil
}
