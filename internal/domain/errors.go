package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound            = errors.New("资源不存在")
	ErrVersionConflict     = errors.New("版本冲突")
	ErrIdempotencyConflict = errors.New("幂等键已用于不同请求")
	ErrInvalidTransition   = errors.New("非法状态迁移")
	ErrFrozen              = errors.New("入库清单已冻结")
	ErrForbidden           = errors.New("角色无权执行此操作")
)

type FieldError struct {
	Issues []ValidationIssue `json:"issues"`
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("请求含有 %d 个字段或业务问题", len(e.Issues))
}

func NewIssue(code, field, message string) ValidationIssue {
	return ValidationIssue{Code: code, Field: field, Message: message}
}

func RequireText(value, field string, issues *[]ValidationIssue) {
	if value == "" {
		*issues = append(*issues, NewIssue("required", field, "字段不能为空"))
	}
}
