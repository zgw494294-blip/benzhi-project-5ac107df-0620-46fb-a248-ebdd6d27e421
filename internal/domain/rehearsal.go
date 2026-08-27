package domain

import "time"

type Rehearsal struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Recorder    string    `json:"recorder"`
	Notes       string    `json:"notes"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
	Revision    int64     `json:"revision"`
}

type RehearsalIssue struct {
	ID                    string `json:"id"`
	ProjectID             string `json:"projectId"`
	RehearsalID           string `json:"rehearsalId"`
	CueID                 string `json:"cueId"`
	Kind                  string `json:"kind"`
	Blocking              bool   `json:"blocking"`
	Note                  string `json:"note"`
	OpenedAgainstRevision int64  `json:"openedAgainstRevision"`
	ResolvedByRevision    int64  `json:"resolvedByRevision,omitempty"`
	ResolutionNote        string `json:"resolutionNote,omitempty"`
	Status                string `json:"status"`
}

const (
	IssuePending  = "待整改"
	IssueResolved = "已解决"
	IssueObserve  = "观察项"
)

func ValidIssueStatus(status string) bool {
	return status == IssuePending || status == IssueResolved || status == IssueObserve
}

func ValidIssueKind(kind string) bool {
	switch kind {
	case "提前", "滞后", "遮挡", "语义":
		return true
	default:
		return false
	}
}
