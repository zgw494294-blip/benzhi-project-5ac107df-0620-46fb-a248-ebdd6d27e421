package service

import (
	"context"
	"fmt"
	"stagecaption/internal/domain"
	"stagecaption/internal/store"
	"time"
)

func (s *Service) RecordRehearsal(ctx context.Context, projectID string, in RehearsalInput) (domain.CaptionProject, []domain.RehearsalIssue, error) {
	recorder := cleanActor(in.Recorder)
	if recorder == "" {
		return domain.CaptionProject{}, nil, fmt.Errorf("%w：排演记录员不能为空", domain.ErrValidation)
	}
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return p, nil, err
	}
	if p.Status != domain.StatusRehearsal {
		return p, nil, domain.ErrInvalidState
	}
	started := in.StartedAt
	if started.IsZero() {
		started = s.Now().Add(-time.Minute)
	}
	rid := newID("reh_")
	issues := make([]domain.RehearsalIssue, 0, len(in.Issues))
	blocking := false
	for _, raw := range in.Issues {
		if !domain.ValidIssueKind(raw.Kind) || trimLimit(raw.CueID, 80) == "" || trimLimit(raw.Note, 500) == "" {
			return p, nil, fmt.Errorf("%w：排演问题的字幕、类型和说明必须有效", domain.ErrValidation)
		}
		i := domain.RehearsalIssue{ID: newID("iss_"), ProjectID: projectID, RehearsalID: rid, CueID: raw.CueID, Kind: raw.Kind, Blocking: raw.Blocking, Note: trimLimit(raw.Note, 500), OpenedAgainstRevision: p.Revision, Status: "待整改"}
		if !raw.Blocking {
			i.Status = "观察项"
		}
		issues = append(issues, i)
		blocking = blocking || raw.Blocking
	}
	cues, err := s.Store.ListCues(ctx, projectID)
	if err != nil {
		return p, nil, err
	}
	owned := make(map[string]bool, len(cues))
	for _, cue := range cues {
		owned[cue.ID] = true
	}
	for _, issue := range issues {
		if !owned[issue.CueID] {
			return p, nil, fmt.Errorf("%w：排演问题引用的字幕不属于当前项目", domain.ErrValidation)
		}
	}
	r := domain.Rehearsal{ID: rid, ProjectID: projectID, Recorder: recorder, Notes: trimLimit(in.Notes, 1000), StartedAt: started.UTC(), CompletedAt: s.Now().UTC()}
	p, err = s.Store.Write(ctx, projectID, in.ExpectedRevision, recorder, "record_rehearsal", false, func(tx *store.Tx, current *domain.CaptionProject) (string, error) {
		if err := tx.AddRehearsal(ctx, r, issues); err != nil {
			return "", err
		}
		to := domain.StatusReview
		if blocking {
			to = domain.StatusRemediation
		}
		if err := domain.RequireTransition(current.Status, to); err != nil {
			return "", err
		}
		current.Status = to
		return fmt.Sprintf("记录排演，发现 %d 个问题", len(issues)), nil
	})
	return p, issues, err
}

func (s *Service) Remediate(ctx context.Context, projectID string, in RemediationInput) (domain.CaptionProject, []domain.QualityFinding, error) {
	result, err := s.BatchRemediate(ctx, projectID, BatchRemediationInput{ExpectedRevision: in.ExpectedRevision, Actor: in.Actor, Scene: in.Scene, LeaseToken: in.LeaseToken, Patches: []RemediationPatch{{Cue: in.Cue, ResolvedIssueIDs: in.ResolvedIssueIDs, ResolutionNote: in.ResolutionNote}}})
	return result.Project, result.TargetedFindings, err
}
