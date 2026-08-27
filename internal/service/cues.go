package service

import (
	"context"
	"fmt"
	"stagecaption/internal/domain"
	"stagecaption/internal/store"
)

func (s *Service) UpsertCue(ctx context.Context, projectID string, in CueInput) (domain.CaptionProject, error) {
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return p, err
	}
	if p.Status != domain.StatusDraft && p.Status != domain.StatusValidation && p.Status != domain.StatusRemediation {
		return p, domain.ErrInvalidState
	}
	actor := cleanActor(in.Actor)
	if actor == "" {
		return p, fmt.Errorf("%w：编辑者不能为空", domain.ErrValidation)
	}
	id := trimLimit(in.ID, 80)
	if id == "" {
		id = newID("cue_")
	}
	cue, err := domain.NormalizeCue(domain.CaptionCue{ID: id, ProjectID: projectID, Scene: in.Scene, Speaker: in.Speaker, Text: in.Text, StartMillis: in.StartMillis, EndMillis: in.EndMillis, Position: in.Position, UpdatedBy: actor}, p.DurationMillis)
	if err != nil {
		return p, err
	}
	return s.Store.Write(ctx, projectID, in.ExpectedRevision, actor, "upsert_cue", false, func(tx *store.Tx, current *domain.CaptionProject) (string, error) {
		if err := tx.RequireLease(ctx, cue.Scene, in.LeaseToken, actor); err != nil {
			return "", err
		}
		if current.Status == domain.StatusDraft {
			if err := domain.RequireTransition(current.Status, domain.StatusValidation); err != nil {
				return "", err
			}
			current.Status = domain.StatusValidation
		}
		if err := tx.UpsertCue(ctx, cue); err != nil {
			return "", err
		}
		return "保存字幕 " + cue.ID, nil
	})
}

func (s *Service) DeleteCue(ctx context.Context, projectID, cueID, scene, token, actor string, expected int64) (domain.CaptionProject, error) {
	actor = cleanActor(actor)
	return s.Store.Write(ctx, projectID, expected, actor, "delete_cue", false, func(tx *store.Tx, p *domain.CaptionProject) (string, error) {
		if p.Status != domain.StatusDraft && p.Status != domain.StatusValidation && p.Status != domain.StatusRemediation {
			return "", domain.ErrInvalidState
		}
		if err := tx.RequireLease(ctx, scene, token, actor); err != nil {
			return "", err
		}
		if err := tx.DeleteCue(ctx, cueID); err != nil {
			return "", err
		}
		return "删除字幕 " + cueID, nil
	})
}
