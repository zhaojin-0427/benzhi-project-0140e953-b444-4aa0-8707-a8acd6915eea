package domain

import (
	"sort"
	"strings"
	"time"
)

func (b *TransferBatch) RegisterPermit(permit CollectionPermit, at time.Time) (Event, error) {
	if err := b.EnsureMutable(); err != nil {
		return Event{}, err
	}
	if b.Status != StatusDraft {
		return Event{}, ErrInvalidTransition
	}
	issues := []ValidationIssue{}
	RequireText(permit.ID, "id", &issues)
	RequireText(permit.PermitNumber, "permitNumber", &issues)
	RequireText(permit.Issuer, "issuer", &issues)
	if permit.ValidFrom.IsZero() {
		issues = append(issues, NewIssue("required", "validFrom", "许可起始时间不能为空"))
	}
	if permit.ValidUntil.Before(permit.ValidFrom) {
		issues = append(issues, NewIssue("invalid_range", "validUntil", "许可终止时间必须晚于起始时间"))
	}
	if len(permit.AllowedMaterialCodes) == 0 {
		issues = append(issues, NewIssue("required", "allowedMaterialCodes", "至少登记一种材料代码"))
	}
	seenCodes := map[string]bool{}
	for i, code := range permit.AllowedMaterialCodes {
		field := "allowedMaterialCodes"
		code = strings.TrimSpace(code)
		if code == "" {
			issues = append(issues, NewIssue("required", field, "材料代码不能为空"))
			continue
		}
		if seenCodes[code] {
			issues = append(issues, NewIssue("duplicate", field, "许可材料代码不能重复"))
		}
		seenCodes[code] = true
		permit.AllowedMaterialCodes[i] = code
	}
	if permit.QuantityLimit < 1 {
		issues = append(issues, NewIssue("invalid", "quantityLimit", "数量上限必须大于零"))
	}
	for _, existing := range b.Permits {
		if existing.PermitNumber == permit.PermitNumber {
			issues = append(issues, NewIssue("duplicate", "permitNumber", "许可编号重复"))
		}
	}
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	permit.BatchID = b.ID
	return NewEvent(EventPermitRegistered, b.ID, b.Version+1, at, permit)
}

func (b *TransferBatch) permitCovers(s Specimen) bool {
	for _, permit := range b.Permits {
		if s.CollectedAt.Before(permit.ValidFrom) || s.CollectedAt.After(permit.ValidUntil) {
			continue
		}
		for _, code := range permit.AllowedMaterialCodes {
			if code == s.MaterialCode {
				return true
			}
		}
	}
	return false
}

// RecalculatePermitQuota 先按许可登记顺序、再按材料代码生成确定性投影；
// 样本也按相同的稳定许可顺序占用有效额度。
func (b *TransferBatch) RecalculatePermitQuota() {
	type quotaKey struct {
		permitIndex int
		code        string
	}
	remaining := map[quotaKey]int{}
	usageIndex := map[quotaKey]int{}
	b.PermitQuota = nil
	b.PermitWarnings = nil
	for permitIndex, permit := range b.Permits {
		codes := append([]string(nil), permit.AllowedMaterialCodes...)
		sort.Strings(codes)
		for _, code := range codes {
			key := quotaKey{permitIndex: permitIndex, code: code}
			remaining[key] = permit.QuantityLimit
			usageIndex[key] = len(b.PermitQuota)
			b.PermitQuota = append(b.PermitQuota, PermitQuotaUsage{
				PermitID: permit.ID, PermitNumber: permit.PermitNumber,
				MaterialCode: code, QuantityLimit: permit.QuantityLimit,
				Remaining: permit.QuantityLimit,
			})
		}
	}
	for _, specimen := range b.Specimens {
		unallocated := specimen.Quantity
		hasEffectivePermit := false
		for permitIndex, permit := range b.Permits {
			if specimen.CollectedAt.Before(permit.ValidFrom) || specimen.CollectedAt.After(permit.ValidUntil) {
				continue
			}
			key := quotaKey{permitIndex: permitIndex, code: specimen.MaterialCode}
			available, covers := remaining[key]
			if !covers {
				continue
			}
			hasEffectivePermit = true
			allocated := unallocated
			if allocated > available {
				allocated = available
			}
			if allocated > 0 {
				remaining[key] -= allocated
				index := usageIndex[key]
				b.PermitQuota[index].UsedQuantity += allocated
				b.PermitQuota[index].Remaining -= allocated
				unallocated -= allocated
			}
			if unallocated == 0 {
				break
			}
		}
		if unallocated > 0 {
			code, message := "permit_quota_exceeded", "有效许可的剩余额度不足"
			if !hasEffectivePermit {
				code, message = "permit_not_effective", "采集时间没有有效许可覆盖该材料"
			}
			b.PermitWarnings = append(b.PermitWarnings, PermitWarning{
				Code: code, MaterialCode: specimen.MaterialCode, SpecimenID: specimen.ID,
				ShortfallQuantity: unallocated, Message: message,
			})
		}
	}
}
