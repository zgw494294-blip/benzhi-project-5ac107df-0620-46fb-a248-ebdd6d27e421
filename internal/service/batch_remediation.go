package service

import (
	"context"
	"fmt"
	"strings"

	"stagecaption/internal/domain"
	"stagecaption/internal/store"
)

const maxRemediationPatches = 50

func (s *Service) BatchRemediate(ctx context.Context, projectID string, in BatchRemediationInput) (RemediationResult, error) {
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return RemediationResult{}, err
	}
	result := RemediationResult{Project: p}
	if p.Status != domain.StatusRemediation {
		return result, domain.ErrInvalidState
	}
	actor, scene := cleanActor(in.Actor), strings.TrimSpace(in.Scene)
	if actor == "" || scene == "" {
		return result, fmt.Errorf("%w：整改编辑者和场次不能为空", domain.ErrValidation)
	}
	if len(in.Patches) == 0 || len(in.Patches) > maxRemediationPatches {
		return result, fmt.Errorf("%w：整改批次须为 1 至 %d 条字幕", domain.ErrValidation, maxRemediationPatches)
	}
	allCues, err := s.Store.ListCues(ctx, projectID)
	if err != nil {
		return result, err
	}
	cueIndex := map[string]int{}
	for i, cue := range allCues {
		cueIndex[cue.ID] = i
	}
	issues, err := s.Store.ListIssues(ctx, projectID)
	if err != nil {
		return result, err
	}
	issueByID := map[string]domain.RehearsalIssue{}
	for _, issue := range issues {
		issueByID[issue.ID] = issue
	}
	seenCue, seenIssue := map[string]bool{}, map[string]bool{}
	updated := make([]domain.CaptionCue, 0, len(in.Patches))
	affected := make([]string, 0, len(in.Patches))
	selected := map[string]bool{}
	for index, patch := range in.Patches {
		id := strings.TrimSpace(patch.Cue.ID)
		if seenCue[id] {
			return result, fmt.Errorf("%w：整改批次包含重复字幕 %s", domain.ErrValidation, id)
		}
		seenCue[id] = true
		at, ok := cueIndex[id]
		if !ok {
			return result, fmt.Errorf("%w：第 %d 条整改字幕不存在", domain.ErrValidation, index+1)
		}
		if allCues[at].Scene != scene {
			return result, fmt.Errorf("%w：整改批次只能包含同一场次字幕", domain.ErrValidation)
		}
		cue, e := domain.NormalizeCue(domain.CaptionCue{ID: id, ProjectID: projectID, Scene: scene, Speaker: patch.Cue.Speaker, Text: patch.Cue.Text, StartMillis: patch.Cue.StartMillis, EndMillis: patch.Cue.EndMillis, Position: patch.Cue.Position, UpdatedBy: actor}, p.DurationMillis)
		if e != nil {
			return result, fmt.Errorf("第 %d 条整改：%w", index+1, e)
		}
		if len(patch.ResolvedIssueIDs) == 0 {
			return result, fmt.Errorf("%w：每条整改字幕至少选择一个待整改问题", domain.ErrValidation)
		}
		for _, issueID := range patch.ResolvedIssueIDs {
			if seenIssue[issueID] {
				return result, fmt.Errorf("%w：问题编号 %s 重复", domain.ErrValidation, issueID)
			}
			seenIssue[issueID] = true
			issue, ok := issueByID[issueID]
			if !ok || issue.ProjectID != projectID {
				return result, fmt.Errorf("%w：问题 %s 不属于当前项目", domain.ErrValidation, issueID)
			}
			if issue.Status != domain.IssuePending {
				return result, fmt.Errorf("%w：问题 %s 不是待整改状态", domain.ErrValidation, issueID)
			}
			if issue.CueID != id {
				return result, fmt.Errorf("%w：问题 %s 与整改字幕不匹配", domain.ErrValidation, issueID)
			}
			selected[issueID] = true
		}
		allCues[at] = cue
		updated = append(updated, cue)
		affected = append(affected, id)
	}
	all, targeted := s.Quality.CheckTargeted(p, allCues, affected)
	result.TargetedFindings = targeted
	if domain.HasBlocking(all) {
		return result, fmt.Errorf("%w：联合目标复验发现阻断规则问题", domain.ErrValidation)
	}
	for _, issue := range issues {
		if !issue.Blocking || selected[issue.ID] {
			continue
		}
		if issue.Status != domain.IssueResolved || issue.ResolvedByRevision <= issue.OpenedAgainstRevision {
			result.RemainingIssueIDs = append(result.RemainingIssueIDs, issue.ID)
		}
	}
	p, err = s.Store.Write(ctx, projectID, in.ExpectedRevision, actor, "batch_remediate_and_retest", false, func(tx *store.Tx, current *domain.CaptionProject) (string, error) {
		if err := tx.RequireLease(ctx, scene, in.LeaseToken, actor); err != nil {
			return "", err
		}
		for _, cue := range updated {
			if err := tx.UpsertCue(ctx, cue); err != nil {
				return "", err
			}
		}
		for _, patch := range in.Patches {
			note := trimLimit(patch.ResolutionNote, 500)
			if note == "" {
				return "", fmt.Errorf("%w：逐问题解决说明不能为空", domain.ErrValidation)
			}
			for _, id := range patch.ResolvedIssueIDs {
				if err := tx.ResolveIssue(ctx, id, note); err != nil {
					return "", err
				}
			}
		}
		if err := tx.ReplaceFindings(ctx, all); err != nil {
			return "", err
		}
		if len(result.RemainingIssueIDs) == 0 {
			if err := domain.RequireTransition(current.Status, domain.StatusReview); err != nil {
				return "", err
			}
			current.Status = domain.StatusReview
		}
		return fmt.Sprintf("批量整改 %d 条字幕并联合复验，解决 %d 个问题", len(updated), len(selected)), nil
	})
	result.Project = p
	return result, err
}
