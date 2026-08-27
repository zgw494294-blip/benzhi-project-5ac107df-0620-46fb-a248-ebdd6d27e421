package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"stagecaption/internal/domain"
)

func (s *Service) QueryProjects(ctx context.Context, filter ProjectQueueFilter) ([]ProjectQueueItem, error) {
	keyword := strings.TrimSpace(filter.Keyword)
	if filter.Keyword != "" && keyword == "" {
		return nil, fmt.Errorf("%w：项目关键词不能只包含空白", domain.ErrValidation)
	}
	if len([]rune(keyword)) > 120 {
		return nil, fmt.Errorf("%w：项目关键词不能超过 120 字", domain.ErrValidation)
	}
	if filter.Status != "" && !domain.ProjectStatus(filter.Status).Valid() {
		return nil, fmt.Errorf("%w：未知项目状态", domain.ErrValidation)
	}
	sortMode := filter.Sort
	if sortMode == "" {
		sortMode = "updated"
	}
	if sortMode == "updatedAt" || sortMode == "lastUpdated" {
		sortMode = "updated"
	}
	if sortMode != "updated" && sortMode != "title" && sortMode != "status" {
		return nil, fmt.Errorf("%w：排序方式必须是 updated、title 或 status", domain.ErrValidation)
	}
	records, err := s.Store.ListProjectQueue(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectQueueItem, 0, len(records))
	for _, record := range records {
		p := record.Project
		if filter.Status != "" && string(p.Status) != filter.Status {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(p.Title), strings.ToLower(keyword)) && !strings.Contains(strings.ToLower(p.ProductionVersion), strings.ToLower(keyword)) {
			continue
		}
		risk := "正常"
		switch {
		case p.Status == domain.StatusRemediation && record.UnresolvedBlockingIssues > 0:
			risk = "阻断整改"
		case p.Status == domain.StatusReview:
			risk = "待独立复核"
		case p.Status == domain.StatusLocked:
			risk = "锁版完成"
		}
		out = append(out, ProjectQueueItem{CaptionProject: p, CueCount: record.CueCount, BlockingFindingCount: record.BlockingFindingCount, UnresolvedBlockingIssueCount: record.UnresolvedBlockingIssues, Risk: risk})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].CaptionProject, out[j].CaptionProject
		switch sortMode {
		case "title":
			if a.Title != b.Title {
				return a.Title < b.Title
			}
		case "status":
			if domain.ProjectStatusRank(a.Status) != domain.ProjectStatusRank(b.Status) {
				return domain.ProjectStatusRank(a.Status) < domain.ProjectStatusRank(b.Status)
			}
		default:
			if !a.UpdatedAt.Equal(b.UpdatedAt) {
				return a.UpdatedAt.After(b.UpdatedAt)
			}
		}
		return a.ID < b.ID
	})
	return out, nil
}

func summarizeText(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > 48 {
		return string(runes[:48]) + "…"
	}
	return string(runes)
}

func (s *Service) QueryFindings(ctx context.Context, projectID string, filter FindingFilter) (FindingView, error) {
	if filter.Severity != "" && filter.Severity != string(domain.SeverityBlocking) && filter.Severity != string(domain.SeverityWarning) {
		return FindingView{}, fmt.Errorf("%w：未知严重程度", domain.ErrValidation)
	}
	if len([]rune(filter.Rule)) > 80 || len([]rune(filter.Scene)) > 80 {
		return FindingView{}, fmt.Errorf("%w：检查筛选条件过长", domain.ErrValidation)
	}
	findings, err := s.Store.ListFindings(ctx, projectID)
	if err != nil {
		return FindingView{}, err
	}
	cues, err := s.Store.ListCues(ctx, projectID)
	if err != nil {
		return FindingView{}, err
	}
	domain.SortFindings(findings, cues)
	byCue := map[string]domain.CaptionCue{}
	sceneSet := map[string]bool{}
	for _, c := range cues {
		byCue[c.ID] = c
		sceneSet[c.Scene] = true
	}
	view := FindingView{Summary: FindingSummary{ByRule: map[string]int{}, ByScene: map[string]int{}}, Rules: []string{}, Scenes: []string{}}
	ruleSet := map[string]bool{}
	for _, finding := range findings {
		if finding.Severity == domain.SeverityBlocking {
			view.Summary.Blocking++
		} else {
			view.Summary.Warning++
		}
		view.Summary.ByRule[finding.RuleCode]++
		ruleSet[finding.RuleCode] = true
		item := FindingItem{QualityFinding: finding, Scope: "项目范围"}
		if finding.CueID != "" {
			if cue, ok := byCue[finding.CueID]; ok {
				item.Scope = "字幕"
				item.Scene = cue.Scene
				item.StartMillis = cue.StartMillis
				item.EndMillis = cue.EndMillis
				item.Position = cue.Position
				item.TextSummary = summarizeText(cue.Text)
				view.Summary.ByScene[cue.Scene]++
			} else {
				item.Invalid = true
				item.Scope = "失效字幕引用"
			}
		} else {
			view.Summary.ByScene["项目范围"]++
			parseFindingRange(&item)
		}
		matched := filter.Severity == "" || string(finding.Severity) == filter.Severity
		matched = matched && (filter.Rule == "" || finding.RuleCode == filter.Rule)
		matched = matched && (filter.Scene == "" || item.Scene == filter.Scene || (filter.Scene == "项目范围" && finding.CueID == ""))
		if matched {
			view.Items = append(view.Items, item)
		}
	}
	view.Summary.Matched = len(view.Items)
	for rule := range ruleSet {
		view.Rules = append(view.Rules, rule)
	}
	sort.Slice(view.Rules, func(i, j int) bool {
		ri, rj := domain.QualityRuleRank(view.Rules[i]), domain.QualityRuleRank(view.Rules[j])
		if ri != rj {
			return ri < rj
		}
		return view.Rules[i] < view.Rules[j]
	})
	for scene := range sceneSet {
		view.Scenes = append(view.Scenes, scene)
	}
	sort.Strings(view.Scenes)
	if view.Summary.ByScene["项目范围"] > 0 {
		view.Scenes = append(view.Scenes, "项目范围")
	}
	return view, nil
}

func parseFindingRange(item *FindingItem) {
	parts := strings.Split(item.ObservedValue, "-")
	if len(parts) != 2 {
		return
	}
	start, e1 := strconv.ParseInt(parts[0], 10, 64)
	end, e2 := strconv.ParseInt(parts[1], 10, 64)
	if e1 == nil && e2 == nil {
		item.RangeStart, item.RangeEnd = start, end
	}
}

func (s *Service) QueryIssues(ctx context.Context, projectID string, filter IssueFilter) (IssueView, error) {
	if filter.Kind != "" && !domain.ValidIssueKind(filter.Kind) {
		return IssueView{}, fmt.Errorf("%w：未知问题类型", domain.ErrValidation)
	}
	if filter.Status != "" && !domain.ValidIssueStatus(filter.Status) {
		return IssueView{}, fmt.Errorf("%w：未知问题状态", domain.ErrValidation)
	}
	if filter.Blocking != "" && filter.Blocking != "true" && filter.Blocking != "false" {
		return IssueView{}, fmt.Errorf("%w：阻断属性必须是 true 或 false", domain.ErrValidation)
	}
	if len([]rune(filter.Scene)) > 80 {
		return IssueView{}, fmt.Errorf("%w：场次筛选不能超过 80 字", domain.ErrValidation)
	}
	issues, err := s.Store.ListIssues(ctx, projectID)
	if err != nil {
		return IssueView{}, err
	}
	cues, err := s.Store.ListCues(ctx, projectID)
	if err != nil {
		return IssueView{}, err
	}
	rehearsals, err := s.Store.ListRehearsals(ctx, projectID)
	if err != nil {
		return IssueView{}, err
	}
	byCue := map[string]domain.CaptionCue{}
	for _, c := range cues {
		byCue[c.ID] = c
	}
	byRehearsal := map[string]domain.Rehearsal{}
	for _, r := range rehearsals {
		byRehearsal[r.ID] = r
	}
	view := IssueView{}
	sceneSet := map[string]bool{}
	for _, issue := range issues {
		switch issue.Status {
		case domain.IssuePending:
			view.Summary.Pending++
		case domain.IssueResolved:
			view.Summary.Resolved++
		case domain.IssueObserve:
			view.Summary.Observations++
		}
		item := IssueItem{RehearsalIssue: issue}
		if r, ok := byRehearsal[issue.RehearsalID]; ok {
			copy := r
			item.Rehearsal = &copy
		}
		if c, ok := byCue[issue.CueID]; ok && c.ProjectID == projectID {
			item.Scene = c.Scene
			item.StartMillis = c.StartMillis
			item.EndMillis = c.EndMillis
			item.Position = c.Position
			item.TextSummary = summarizeText(c.Text)
			item.Executable = true
			sceneSet[c.Scene] = true
		}
		matched := filter.Scene == "" || item.Scene == filter.Scene
		matched = matched && (filter.Kind == "" || issue.Kind == filter.Kind)
		matched = matched && (filter.Status == "" || issue.Status == filter.Status)
		matched = matched && (filter.Blocking == "" || strconv.FormatBool(issue.Blocking) == filter.Blocking)
		if matched {
			view.Items = append(view.Items, item)
		}
	}
	view.Summary.Matched = len(view.Items)
	for scene := range sceneSet {
		view.Scenes = append(view.Scenes, scene)
	}
	sort.Strings(view.Scenes)
	view.ReplayWindows = makeReplayWindows(view.Items)
	return view, nil
}

func makeReplayWindows(items []IssueItem) []ReplayWindow {
	type candidate struct {
		scene      string
		start, end int64
		issue, cue string
	}
	var candidates []candidate
	for _, item := range items {
		if item.Executable && item.Blocking && item.Status == domain.IssuePending {
			candidates = append(candidates, candidate{item.Scene, item.StartMillis, item.EndMillis, item.ID, item.CueID})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].scene != candidates[j].scene {
			return candidates[i].scene < candidates[j].scene
		}
		if candidates[i].start != candidates[j].start {
			return candidates[i].start < candidates[j].start
		}
		return candidates[i].issue < candidates[j].issue
	})
	var out []ReplayWindow
	cueSets := []map[string]bool{}
	for _, c := range candidates {
		n := len(out)
		if n == 0 || out[n-1].Scene != c.scene || c.start > out[n-1].EndMillis+2000 {
			out = append(out, ReplayWindow{Scene: c.scene, StartMillis: c.start, EndMillis: c.end, IssueIDs: []string{c.issue}})
			cueSets = append(cueSets, map[string]bool{c.cue: true})
		} else {
			if c.end > out[n-1].EndMillis {
				out[n-1].EndMillis = c.end
			}
			out[n-1].IssueIDs = append(out[n-1].IssueIDs, c.issue)
			cueSets[n-1][c.cue] = true
		}
	}
	for i := range out {
		out[i].CueCount = len(cueSets[i])
	}
	return out
}

func (s *Service) CompareRevisions(ctx context.Context, projectID string, from, to int64) (RevisionComparison, error) {
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return RevisionComparison{}, err
	}
	if from < 1 || to < 1 || from > to || to > p.Revision {
		return RevisionComparison{}, fmt.Errorf("%w：修订区间不存在或顺序倒置", domain.ErrValidation)
	}
	a, err := s.Store.GetSnapshot(ctx, projectID, from)
	if err != nil {
		return RevisionComparison{}, fmt.Errorf("%w：起始修订快照不存在", domain.ErrValidation)
	}
	b, err := s.Store.GetSnapshot(ctx, projectID, to)
	if err != nil {
		return RevisionComparison{}, fmt.Errorf("%w：结束修订快照不存在", domain.ErrValidation)
	}
	diffs, err := domain.CompareSnapshots(a, b)
	if err != nil {
		return RevisionComparison{}, err
	}
	if len(diffs) > 2000 {
		return RevisionComparison{}, fmt.Errorf("%w：修订差异超过 2000 条，请缩小复核区间", domain.ErrValidation)
	}
	return RevisionComparison{FromRevision: from, ToRevision: to, Differences: diffs}, nil
}

func (s *Service) DefaultComparison(ctx context.Context, projectID string) (RevisionComparison, error) {
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return RevisionComparison{}, err
	}
	from := int64(1)
	rehearsals, err := s.Store.ListRehearsals(ctx, projectID)
	if err != nil {
		return RevisionComparison{}, err
	}
	if len(rehearsals) > 0 {
		from = rehearsals[len(rehearsals)-1].Revision
	}
	return s.CompareRevisions(ctx, projectID, from, p.Revision)
}
