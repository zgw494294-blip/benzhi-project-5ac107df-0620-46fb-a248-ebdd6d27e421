package domain

import "fmt"

type ProjectStatus string

const (
	StatusDraft       ProjectStatus = "草稿"
	StatusValidation  ProjectStatus = "待校验"
	StatusRehearsal   ProjectStatus = "待排演"
	StatusRemediation ProjectStatus = "整改中"
	StatusReview      ProjectStatus = "待复核"
	StatusLocked      ProjectStatus = "已锁版"
)

var ProjectStatuses = []ProjectStatus{StatusDraft, StatusValidation, StatusRehearsal, StatusRemediation, StatusReview, StatusLocked}

func ProjectStatusRank(status ProjectStatus) int {
	for i, candidate := range ProjectStatuses {
		if status == candidate {
			return i
		}
	}
	return len(ProjectStatuses)
}

func (s ProjectStatus) Valid() bool {
	switch s {
	case StatusDraft, StatusValidation, StatusRehearsal, StatusRemediation, StatusReview, StatusLocked:
		return true
	default:
		return false
	}
}

func CanTransition(from, to ProjectStatus) bool {
	switch from {
	case StatusDraft:
		return to == StatusValidation
	case StatusValidation:
		return to == StatusRehearsal
	case StatusRehearsal:
		return to == StatusRemediation || to == StatusReview
	case StatusRemediation:
		return to == StatusReview
	case StatusReview:
		return to == StatusRemediation || to == StatusLocked
	default:
		return false
	}
}

func RequireTransition(from, to ProjectStatus) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("%w：不能从%s迁移到%s", ErrInvalidState, from, to)
	}
	return nil
}
