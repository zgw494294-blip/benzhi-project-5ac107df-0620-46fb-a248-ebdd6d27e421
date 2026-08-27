package service

import (
	"context"
	"fmt"
	"stagecaption/internal/domain"
	"stagecaption/internal/store"
)

type LockGateError struct{ Gate LockGate }

func (e *LockGateError) Error() string { return "锁版门禁未通过，请处理全部阻断原因" }
func (e *LockGateError) Unwrap() error {
	for _, item := range e.Gate.Items {
		if item.Code == "independent_reviewer" && !item.Passed {
			return domain.ErrReviewerEditor
		}
	}
	return domain.ErrValidation
}

func cloneLockGate(gate LockGate) LockGate {
	copy := gate
	copy.Items = append([]GateItem(nil), gate.Items...)
	return copy
}

func (s *Service) CheckLockGate(ctx context.Context, projectID, reviewer string) (LockGate, error) {
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return LockGate{}, err
	}
	reviewer = cleanActor(reviewer)
	cacheKey := fmt.Sprintf("%s\x00%s\x00%d", projectID, reviewer, p.Revision)
	s.gateMu.RLock()
	cached, ok := s.gateCache[cacheKey]
	s.gateMu.RUnlock()
	if ok {
		return cloneLockGate(cached), nil
	}
	gate := LockGate{Passed: true, ToRevision: p.Revision}
	add := func(code, label string, passed bool, message string) {
		gate.Items = append(gate.Items, GateItem{Code: code, Label: label, Passed: passed, Message: message})
		gate.Passed = gate.Passed && passed
	}
	add("independent_reviewer", "复核员独立", reviewer != "" && reviewer != p.LastEditor, func() string {
		if reviewer == "" {
			return "请填写复核员"
		}
		if reviewer == p.LastEditor {
			return "复核员不能是最后编辑者"
		}
		return "复核员与最后编辑者不同"
	}())
	leases, err := s.Store.ListActiveLeases(ctx, projectID)
	if err != nil {
		return LockGate{}, err
	}
	add("leases_clear", "编辑租约已清空", len(leases) == 0, func() string {
		if len(leases) == 0 {
			return "没有活动编辑租约"
		}
		return fmt.Sprintf("仍有 %d 个活动编辑租约", len(leases))
	}())
	cues, err := s.Store.ListCues(ctx, projectID)
	if err != nil {
		return LockGate{}, err
	}
	findings := s.Quality.Check(p, cues)
	blocking := 0
	for _, f := range findings {
		if f.Severity == domain.SeverityBlocking {
			blocking++
		}
	}
	add("quality_clear", "无阻断规则结果", blocking == 0, fmt.Sprintf("当前复算得到 %d 个阻断规则结果", blocking))
	issues, err := s.Store.ListIssues(ctx, projectID)
	if err != nil {
		return LockGate{}, err
	}
	unresolved := 0
	for _, issue := range issues {
		if issue.Blocking && (issue.Status != domain.IssueResolved || issue.ResolvedByRevision <= issue.OpenedAgainstRevision) {
			unresolved++
		}
	}
	add("issues_resolved", "阻断排演问题由较新修订解决", unresolved == 0, fmt.Sprintf("仍有 %d 个阻断排演问题不满足解决修订条件", unresolved))
	rehearsals, err := s.Store.ListRehearsals(ctx, projectID)
	if err != nil {
		return LockGate{}, err
	}
	from := int64(1)
	if len(rehearsals) > 0 {
		from = rehearsals[len(rehearsals)-1].Revision
	}
	gate.FromRevision = from
	fromSnapshot, fromErr := s.Store.GetSnapshot(ctx, projectID, from)
	toSnapshot, toErr := s.Store.GetSnapshot(ctx, projectID, p.Revision)
	snapshotsOK := fromErr == nil && toErr == nil && fromSnapshot.Project.ID == projectID && toSnapshot.Project.ID == projectID
	add("snapshots_readable", "复核区间快照可读取", snapshotsOK, func() string {
		if snapshotsOK {
			return fmt.Sprintf("修订 r%d 至 r%d 快照可读取", from, p.Revision)
		}
		return "复核区间快照缺失或无法读取"
	}())
	s.gateMu.Lock()
	s.gateCache[cacheKey] = cloneLockGate(gate)
	s.gateMu.Unlock()
	return gate, nil
}

func (s *Service) Review(ctx context.Context, projectID string, in ReviewInput) (domain.CaptionProject, *domain.ReleaseBundle, error) {
	reviewer := cleanActor(in.Reviewer)
	if reviewer == "" {
		return domain.CaptionProject{}, nil, fmt.Errorf("%w：复核员不能为空", domain.ErrValidation)
	}
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return p, nil, err
	}
	if p.Status != domain.StatusReview {
		return p, nil, domain.ErrInvalidState
	}
	if in.Decision != "lock" && in.Decision != "return" {
		return p, nil, fmt.Errorf("%w：复核决定必须是 lock 或 return", domain.ErrValidation)
	}
	if in.Decision == "return" {
		p, err = s.Store.Write(ctx, projectID, in.ExpectedRevision, reviewer, "review_return", false, func(tx *store.Tx, current *domain.CaptionProject) (string, error) {
			if err := domain.RequireTransition(current.Status, domain.StatusRemediation); err != nil {
				return "", err
			}
			current.Status = domain.StatusRemediation
			return "独立复核退回：" + trimLimit(in.Note, 500), nil
		})
		return p, nil, err
	}
	if p.Revision != in.ExpectedRevision {
		return p, nil, &domain.ConflictError{CurrentRevision: p.Revision}
	}
	if err = s.Store.SetBarrier(ctx, projectID, true); err != nil {
		return p, nil, err
	}
	defer s.Store.SetBarrier(context.Background(), projectID, false)
	gate, err := s.CheckLockGate(ctx, projectID, reviewer)
	if err != nil {
		return p, nil, err
	}
	if !gate.Passed {
		return p, nil, &LockGateError{Gate: gate}
	}
	cues, err := s.Store.ListCues(ctx, projectID)
	if err != nil {
		return p, nil, err
	}
	var release domain.ReleaseBundle
	p, err = s.Store.Write(ctx, projectID, in.ExpectedRevision, reviewer, "lock", true, func(tx *store.Tx, current *domain.CaptionProject) (string, error) {
		if err := domain.RequireTransition(current.Status, domain.StatusLocked); err != nil {
			return "", err
		}
		current.Status = domain.StatusLocked
		locked := *current
		locked.Revision++
		locked.LastEditor = reviewer
		locked.UpdatedAt = s.Now().UTC()
		files, e := makeBundle(locked, cues, reviewer, newID("rel_"), domain.ReleaseBundle{})
		if e != nil {
			return "", e
		}
		release = files.Release
		release.IssuedAt = s.Now().UTC()
		if e = tx.SaveRelease(ctx, release); e != nil {
			return "", e
		}
		return "独立复核通过并锁版：" + trimLimit(in.Note, 500), nil
	})
	if err != nil {
		return p, nil, err
	}
	return p, &release, nil
}
