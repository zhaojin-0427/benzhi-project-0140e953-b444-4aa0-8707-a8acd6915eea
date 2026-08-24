package domain

import (
	"fmt"
	"sort"
	"time"
)

type ReceivedSpecimen struct {
	SpecimenID            string `json:"specimenId"`
	ContainerCode         string `json:"containerCode"`
	SealCode              string `json:"sealCode"`
	SealCondition         string `json:"sealCondition"`
	PreservationCondition string `json:"preservationCondition"`
	Quantity              int    `json:"quantity"`
}

func (b *TransferBatch) InspectArrival(received []ReceivedSpecimen, at time.Time, idFactory func() string) (Event, error) {
	if err := b.EnsureMutable(); err != nil {
		return Event{}, err
	}
	if b.Status != StatusCustodyInTransit {
		return Event{}, ErrInvalidTransition
	}
	if len(b.Handoffs) == 0 {
		return Event{}, &FieldError{Issues: []ValidationIssue{NewIssue("handoff_missing", "handoffs", "至少需要一次已确认交接")}}
	}
	issues := validateReceived(received)
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	discrepancies := b.compareArrival(received, at, idFactory)
	return NewEvent(EventArrivalInspected, b.ID, b.Version+1, at, ArrivalInspectedData{Discrepancies: discrepancies})
}

func validateReceived(received []ReceivedSpecimen) []ValidationIssue {
	issues := []ValidationIssue{}
	if len(received) == 0 {
		issues = append(issues, NewIssue("required", "received", "实收样本清单不能为空"))
	}
	seen := map[string]bool{}
	for _, item := range received {
		if item.SpecimenID == "" {
			issues = append(issues, NewIssue("required", "received.specimenId", "实收样本编号不能为空"))
		}
		if seen[item.SpecimenID] {
			issues = append(issues, ValidationIssue{Code: "duplicate", Field: "received.specimenId", SpecimenID: item.SpecimenID, Message: "实收样本编号重复"})
		}
		seen[item.SpecimenID] = true
		if item.ContainerCode == "" {
			issues = append(issues, ValidationIssue{Code: "required", Field: "received.containerCode", SpecimenID: item.SpecimenID, Message: "实收容器编号不能为空"})
		}
		if item.SealCode == "" || item.SealCondition == "" {
			issues = append(issues, ValidationIssue{Code: "required", Field: "received.seal", SpecimenID: item.SpecimenID, Message: "实收封签及其状态不能为空"})
		}
		if item.PreservationCondition == "" {
			issues = append(issues, ValidationIssue{Code: "required", Field: "received.preservationCondition", SpecimenID: item.SpecimenID, Message: "实收保存条件不能为空"})
		}
		if item.Quantity < 1 {
			issues = append(issues, ValidationIssue{Code: "invalid", Field: "received.quantity", SpecimenID: item.SpecimenID, Message: "实收数量必须大于零"})
		}
	}
	return issues
}

func (b *TransferBatch) compareArrival(received []ReceivedSpecimen, at time.Time, idFactory func() string) []Discrepancy {
	byID := make(map[string]ReceivedSpecimen, len(received))
	for _, item := range received {
		byID[item.SpecimenID] = item
	}
	result := []Discrepancy{}
	add := func(specimenID, category, description string) {
		result = append(result, Discrepancy{ID: idFactory(), BatchID: b.ID, SpecimenID: specimenID, Category: category, Description: description, Status: DiscrepancyOpen})
	}
	for _, expected := range b.Specimens {
		actual, ok := byID[expected.ID]
		if !ok {
			add(expected.ID, "missing", "计划样本未到站")
			continue
		}
		if actual.ContainerCode != expected.ContainerCode {
			add(expected.ID, "container", "容器编号与计划不符")
		}
		if actual.SealCode != expected.SealCode || actual.SealCondition != "intact" {
			add(expected.ID, "seal", "封签编号不符或封签不完整")
		}
		if actual.PreservationCondition != expected.PreservationRequirement {
			add(expected.ID, "preservation", "保存条件与要求不符")
		}
		if actual.Quantity != expected.Quantity {
			add(expected.ID, "quantity", fmt.Sprintf("实收数量 %d 与计划数量 %d 不符", actual.Quantity, expected.Quantity))
		}
		delete(byID, expected.ID)
	}
	unexpectedIDs := make([]string, 0, len(byID))
	for id := range byID {
		unexpectedIDs = append(unexpectedIDs, id)
	}
	sort.Strings(unexpectedIDs)
	for _, id := range unexpectedIDs {
		add(id, "unexpected", "收到计划外样本")
	}
	return result
}

func (b *TransferBatch) SubmitRemediation(discrepancyID, note, evidence string, at time.Time) (Event, error) {
	return b.SubmitRemediationBy(discrepancyID, note, evidence, "legacy-operator", at)
}

func (b *TransferBatch) SubmitRemediationBy(discrepancyID, note, evidence, submittedBy string, at time.Time) (Event, error) {
	if err := b.EnsureMutable(); err != nil {
		return Event{}, err
	}
	if b.Status != StatusRemediationRequired {
		return Event{}, ErrInvalidTransition
	}
	issues := []ValidationIssue{}
	RequireText(note, "remediationNote", &issues)
	RequireText(evidence, "evidenceDigest", &issues)
	RequireText(submittedBy, "submittedBy", &issues)
	found := false
	revision := 1
	for _, item := range b.Discrepancies {
		if item.ID == discrepancyID {
			found = true
			if item.Status == DiscrepancyClosed {
				issues = append(issues, NewIssue("already_closed", "discrepancyId", "问题已经关闭"))
			}
			if item.Status == DiscrepancyRemediated {
				issues = append(issues, NewIssue("review_pending", "discrepancyId", "当前整改修订尚待复验"))
			}
			revision = len(item.Revisions) + 1
		}
	}
	if !found {
		return Event{}, ErrNotFound
	}
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	return NewEvent(EventRemediationSubmitted, b.ID, b.Version+1, at, RemediationSubmittedData{DiscrepancyID: discrepancyID, Revision: revision, SubmittedBy: submittedBy, Note: note, EvidenceDigest: evidence})
}

func (b *TransferBatch) ReviewDiscrepancy(discrepancyID, reviewer string, approved bool, at time.Time) (Event, error) {
	revision := 0
	for _, item := range b.Discrepancies {
		if item.ID == discrepancyID {
			revision = len(item.Revisions)
		}
	}
	return b.ReviewRevision(discrepancyID, revision, reviewer, approved, "legacy review", at)
}

func (b *TransferBatch) ReviewRevision(discrepancyID string, revision int, reviewer string, approved bool, opinion string, at time.Time) (Event, error) {
	if err := b.EnsureMutable(); err != nil {
		return Event{}, err
	}
	if b.Status != StatusRemediationRequired {
		return Event{}, ErrInvalidTransition
	}
	issues := []ValidationIssue{}
	RequireText(reviewer, "reviewedBy", &issues)
	RequireText(opinion, "opinion", &issues)
	if revision < 1 {
		issues = append(issues, NewIssue("invalid", "revision", "复验必须指定有效的整改修订号"))
	}
	found := false
	for _, item := range b.Discrepancies {
		if item.ID == discrepancyID {
			found = true
			if item.Status != DiscrepancyRemediated {
				return Event{}, ErrInvalidTransition
			}
			latest := len(item.Revisions)
			if revision != latest {
				issues = append(issues, ValidationIssue{Code: "stale_revision", Field: "revision", Expected: fmt.Sprintf("%d", latest), Actual: fmt.Sprintf("%d", revision), Message: "只能复验当前最新整改修订"})
			} else if latest > 0 && item.Revisions[latest-1].SubmittedBy == reviewer {
				issues = append(issues, ValidationIssue{Code: "reviewer_conflict", Field: "reviewedBy", Actual: reviewer, Message: "整改提交人不能复验自己的当前修订"})
			}
		}
	}
	if !found {
		return Event{}, ErrNotFound
	}
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	return NewEvent(EventDiscrepancyReviewed, b.ID, b.Version+1, at, DiscrepancyReviewedData{DiscrepancyID: discrepancyID, Revision: revision, Approved: approved, ReviewedBy: reviewer, Opinion: opinion})
}

func (b *TransferBatch) RecalculateDiscrepancyActions() {
	for i := range b.Discrepancies {
		switch b.Discrepancies[i].Status {
		case DiscrepancyOpen, DiscrepancyRejected:
			b.Discrepancies[i].LatestActions = []string{"submit_remediation"}
		case DiscrepancyRemediated:
			b.Discrepancies[i].LatestActions = []string{"review_latest_revision"}
		default:
			b.Discrepancies[i].LatestActions = []string{}
		}
	}
}

func (b *TransferBatch) ReverifyArrival(at time.Time) (Event, error) {
	if err := b.EnsureMutable(); err != nil {
		return Event{}, err
	}
	if b.Status != StatusRemediationRequired {
		return Event{}, ErrInvalidTransition
	}
	issues := []ValidationIssue{}
	for _, item := range b.Discrepancies {
		if item.Status != DiscrepancyClosed {
			issues = append(issues, ValidationIssue{Code: "discrepancy_open", Field: "discrepancies", SpecimenID: item.SpecimenID, Message: "仍有问题未通过复验"})
		}
	}
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	return NewEvent(EventArrivalReverified, b.ID, b.Version+1, at, struct {
		Closed int `json:"closed"`
	}{Closed: len(b.Discrepancies)})
}
