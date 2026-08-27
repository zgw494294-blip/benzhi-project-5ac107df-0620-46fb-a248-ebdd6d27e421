package domain

import "errors"

var (
	ErrNotFound       = errors.New("记录不存在")
	ErrRevision       = errors.New("项目修订号冲突")
	ErrInvalidState   = errors.New("当前流程状态不允许该操作")
	ErrLocked         = errors.New("项目已锁版，不能再编辑")
	ErrLeaseRequired  = errors.New("缺少有效的场次编辑租约")
	ErrWriteBarrier   = errors.New("项目正在复核，已禁止写入")
	ErrValidation     = errors.New("输入校验失败")
	ErrReviewerEditor = errors.New("最后编辑者不能担任锁版复核员")
)

type ConflictError struct{ CurrentRevision int64 }

func (e *ConflictError) Error() string { return "项目修订号冲突" }
func (e *ConflictError) Unwrap() error { return ErrRevision }
