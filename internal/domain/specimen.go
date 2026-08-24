package domain

import (
	"sort"
	"strings"
	"time"
)

type DepartureIssueCounts struct {
	Permit         int `json:"permit"`
	Identification int `json:"identification"`
	Preservation   int `json:"preservation"`
}

type DepartureReadinessReport struct {
	Ready  bool                 `json:"ready"`
	Issues []ValidationIssue    `json:"issues"`
	Counts DepartureIssueCounts `json:"counts"`
}

func (b *TransferBatch) RegisterSpecimen(specimen Specimen, at time.Time) (Event, error) {
	if err := b.EnsureMutable(); err != nil {
		return Event{}, err
	}
	if b.Status != StatusDraft {
		return Event{}, ErrInvalidTransition
	}
	issues := []ValidationIssue{}
	RequireText(specimen.ID, "id", &issues)
	RequireText(specimen.MaterialCode, "materialCode", &issues)
	RequireText(specimen.SourceDescription, "sourceDescription", &issues)
	RequireText(specimen.ContainerCode, "containerCode", &issues)
	RequireText(specimen.SealCode, "sealCode", &issues)
	RequireText(specimen.PreservationRequirement, "preservationRequirement", &issues)
	if specimen.CollectedAt.IsZero() {
		issues = append(issues, NewIssue("required", "collectedAt", "采集时间不能为空"))
	}
	if specimen.Quantity < 1 {
		issues = append(issues, NewIssue("invalid", "quantity", "样本数量必须大于零"))
	}
	for _, existing := range b.Specimens {
		if existing.ID == specimen.ID {
			issues = append(issues, NewIssue("duplicate", "id", "样本编号重复"))
		}
		if existing.ContainerCode == specimen.ContainerCode {
			issues = append(issues, NewIssue("duplicate", "containerCode", "容器编号重复"))
		}
		if existing.SealCode == specimen.SealCode {
			issues = append(issues, NewIssue("duplicate", "sealCode", "封签编号重复"))
		}
	}
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	specimen.BatchID = b.ID
	return NewEvent(EventSpecimenRegistered, b.ID, b.Version+1, at, specimen)
}

func (b *TransferBatch) ValidateDeparture() []ValidationIssue {
	return b.DepartureReadiness().Issues
}

func (b *TransferBatch) DepartureReadiness() DepartureReadinessReport {
	issues := []ValidationIssue{}
	if len(b.Permits) == 0 {
		issues = append(issues, NewIssue("permit_missing", "permits", "未登记采集许可"))
	}
	if len(b.Specimens) == 0 {
		issues = append(issues, NewIssue("specimen_missing", "specimens", "未登记样本"))
	}
	b.RecalculatePermitQuota()
	for _, specimen := range b.Specimens {
		if strings.TrimSpace(specimen.MaterialCode) == "" {
			issues = append(issues, ValidationIssue{Code: "label_incomplete", Field: "materialCode", SpecimenID: specimen.ID, Message: "样本材料代码不能为空"})
		}
		if strings.TrimSpace(specimen.ContainerCode) == "" {
			issues = append(issues, ValidationIssue{Code: "container_missing", Field: "containerCode", SpecimenID: specimen.ID, Message: "样本容器编号不能为空"})
		}
		if strings.TrimSpace(specimen.SealCode) == "" {
			issues = append(issues, ValidationIssue{Code: "seal_missing", Field: "sealCode", SpecimenID: specimen.ID, Message: "样本封签编号不能为空"})
		}
		if strings.TrimSpace(specimen.PreservationRequirement) == "" {
			issues = append(issues, ValidationIssue{Code: "preservation_missing", Field: "preservationRequirement", SpecimenID: specimen.ID, Message: "样本保存要求不能为空"})
		}
	}
	for _, warning := range b.PermitWarnings {
		code := "quantity_limit"
		if warning.Code == "permit_not_effective" {
			code = "permit_scope"
		}
		issues = append(issues, ValidationIssue{Code: code, Field: "quantity", SpecimenID: warning.SpecimenID, ShortfallQuantity: warning.ShortfallQuantity, Message: warning.Message + "：" + warning.MaterialCode})
	}
	containers, seals := map[string]string{}, map[string]string{}
	for _, specimen := range b.Specimens {
		if previous, exists := containers[specimen.ContainerCode]; exists && specimen.ContainerCode != "" {
			issues = append(issues, ValidationIssue{Code: "container_duplicate", Field: "containerCode", SpecimenID: specimen.ID, Expected: "不同于样本 " + previous, Actual: specimen.ContainerCode, Message: "容器编号在批次内重复"})
		} else {
			containers[specimen.ContainerCode] = specimen.ID
		}
		if previous, exists := seals[specimen.SealCode]; exists && specimen.SealCode != "" {
			issues = append(issues, ValidationIssue{Code: "seal_duplicate", Field: "sealCode", SpecimenID: specimen.ID, Expected: "不同于样本 " + previous, Actual: specimen.SealCode, Message: "封签编号在批次内重复"})
		} else {
			seals[specimen.SealCode] = specimen.ID
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		if issues[i].Field != issues[j].Field {
			return issues[i].Field < issues[j].Field
		}
		return issues[i].SpecimenID < issues[j].SpecimenID
	})
	report := DepartureReadinessReport{Ready: len(issues) == 0, Issues: issues}
	for _, issue := range issues {
		switch issue.Code {
		case "permit_missing", "permit_scope", "quantity_limit":
			report.Counts.Permit++
		case "preservation_missing":
			report.Counts.Preservation++
		default:
			report.Counts.Identification++
		}
	}
	return report
}

func (b *TransferBatch) VerifyDeparture(at time.Time) (Event, error) {
	if err := b.EnsureMutable(); err != nil {
		return Event{}, err
	}
	if b.Status != StatusDraft {
		return Event{}, ErrInvalidTransition
	}
	issues := b.ValidateDeparture()
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	return NewEvent(EventDepartureVerified, b.ID, b.Version+1, at, DepartureVerifiedData{IssuesChecked: len(b.Specimens)})
}
