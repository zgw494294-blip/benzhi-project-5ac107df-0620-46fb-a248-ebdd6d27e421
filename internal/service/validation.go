package service

import (
	"context"
	"errors"
	"fmt"
	"stagecaption/internal/domain"
	"stagecaption/internal/store"
)

func (s *Service) Validate(ctx context.Context, projectID string, in ValidateInput) (domain.CaptionProject, []domain.QualityFinding, error) {
	actor := cleanActor(in.Actor)
	if actor == "" {
		return domain.CaptionProject{}, nil, fmt.Errorf("%w：校验操作者不能为空", domain.ErrValidation)
	}
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return p, nil, err
	}
	if p.Status != domain.StatusValidation {
		return p, nil, domain.ErrInvalidState
	}
	cues, err := s.Store.ListCues(ctx, projectID)
	if err != nil {
		return p, nil, err
	}
	findings := s.Quality.Check(p, cues)
	expected := in.ExpectedRevision
	for attempt := 0; attempt < 2; attempt++ {
		p, err = s.Store.Write(ctx, projectID, expected, actor, "quality_check", false, func(tx *store.Tx, current *domain.CaptionProject) (string, error) {
			if err := tx.ReplaceFindings(ctx, findings); err != nil {
				return "", err
			}
			if !domain.HasBlocking(findings) {
				if err := domain.RequireTransition(current.Status, domain.StatusRehearsal); err != nil {
					return "", err
				}
				current.Status = domain.StatusRehearsal
			}
			return fmt.Sprintf("完成全量规则检查，共 %d 项", len(findings)), nil
		})
		if err == nil {
			return p, findings, nil
		}
		var conflict *domain.ConflictError
		if !errors.As(err, &conflict) {
			return p, findings, err
		}
		expected = conflict.CurrentRevision
	}
	return p, findings, err
}
