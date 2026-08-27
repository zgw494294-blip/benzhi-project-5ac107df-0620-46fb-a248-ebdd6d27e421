package service

import (
	"context"
	"fmt"
	"time"

	"stagecaption/internal/domain"
	"stagecaption/internal/store"
)

func (s *Service) CreateProject(ctx context.Context, in CreateProjectInput) (domain.CaptionProject, error) {
	actor := cleanActor(in.Actor)
	if actor == "" {
		return domain.CaptionProject{}, fmt.Errorf("%w：建档人不能为空", domain.ErrValidation)
	}
	p, err := domain.NewProject(newID("prj_"), in.Title, in.ProductionVersion, in.FrameRate, in.DurationMillis, in.TimeOrigin, actor, s.Now())
	if err != nil {
		return p, err
	}
	err = s.Store.CreateProject(ctx, p)
	return p, err
}

func (s *Service) ListProjects(ctx context.Context) ([]domain.CaptionProject, error) {
	return s.Store.ListProjects(ctx)
}

func (s *Service) GetWorkspace(ctx context.Context, id string) (Workspace, error) {
	return s.GetWorkspaceFiltered(ctx, id, FindingFilter{}, IssueFilter{}, 0, 0, "")
}

func (s *Service) GetWorkspaceFiltered(ctx context.Context, id string, findingFilter FindingFilter, issueFilter IssueFilter, fromRevision, toRevision int64, reviewer string) (Workspace, error) {
	p, err := s.Store.GetProject(context.WithoutCancel(ctx), id)
	if err != nil {
		return Workspace{}, err
	}
	cues, err := s.Store.ListCues(context.WithoutCancel(ctx), id)
	if err != nil {
		return Workspace{}, err
	}
	findings, err := s.Store.ListFindings(context.WithoutCancel(ctx), id)
	if err != nil {
		return Workspace{}, err
	}
	rehearsals, err := s.Store.ListRehearsals(context.WithoutCancel(ctx), id)
	if err != nil {
		return Workspace{}, err
	}
	issues, err := s.Store.ListIssues(context.WithoutCancel(ctx), id)
	if err != nil {
		return Workspace{}, err
	}
	audits, err := s.Store.ListAudits(context.WithoutCancel(ctx), id)
	if err != nil {
		return Workspace{}, err
	}
	w := Workspace{Project: p, Cues: cues, Findings: findings, Rehearsals: rehearsals, Issues: issues, Audits: audits}
	w.FindingView, err = s.QueryFindings(context.WithoutCancel(ctx), id, findingFilter)
	if err != nil {
		return Workspace{}, err
	}
	w.IssueView, err = s.QueryIssues(context.WithoutCancel(ctx), id, issueFilter)
	if err != nil {
		return Workspace{}, err
	}
	if r, e := s.Store.GetRelease(context.WithoutCancel(ctx), id); e == nil {
		w.Release = &r
	}
	if p.Revision > 1 {
		if d, e := s.Store.Diff(context.WithoutCancel(ctx), id, 1, p.Revision); e == nil {
			w.Diff = &d
		}
	}
	if fromRevision == 0 && toRevision == 0 {
		if comparison, e := s.DefaultComparison(context.WithoutCancel(ctx), id); e == nil {
			w.Comparison = &comparison
		}
	} else {
		if fromRevision == 0 || toRevision == 0 {
			return Workspace{}, fmt.Errorf("%w：修订区间必须同时提供起止修订", domain.ErrValidation)
		}
		comparison, e := s.CompareRevisions(context.WithoutCancel(ctx), id, fromRevision, toRevision)
		if e != nil {
			return Workspace{}, e
		}
		w.Comparison = &comparison
	}
	if p.Status == domain.StatusReview {
		gate, e := s.CheckLockGate(context.WithoutCancel(ctx), id, reviewer)
		if e != nil {
			return Workspace{}, e
		}
		w.LockGate = &gate
	}
	return w, nil
}

func (s *Service) AcquireLease(ctx context.Context, projectID string, in LeaseInput) (string, time.Time, error) {
	actor := cleanActor(in.Actor)
	scene := trimLimit(in.Scene, 80)
	if actor == "" || scene == "" {
		return "", time.Time{}, fmt.Errorf("%w：场次和编辑者不能为空", domain.ErrValidation)
	}
	ttl := in.TTLSeconds
	if ttl <= 0 {
		ttl = 300
	}
	if ttl > 900 {
		ttl = 900
	}
	token := newID("lease_")
	lease, err := s.Store.AcquireLease(ctx, storeLease(projectID, scene, token, actor, s.Now().Add(time.Duration(ttl)*time.Second)))
	return lease.Token, lease.ExpiresAt, err
}

func storeLease(projectID, scene, token, actor string, expires time.Time) store.Lease {
	return store.Lease{ProjectID: projectID, Scene: scene, Token: token, Holder: actor, ExpiresAt: expires}
}

func (s *Service) ReleaseLease(ctx context.Context, projectID, scene, token string) error {
	return s.Store.ReleaseLease(ctx, projectID, scene, token)
}
