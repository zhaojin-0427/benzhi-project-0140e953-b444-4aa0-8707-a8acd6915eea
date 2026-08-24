package domain

import "time"

func (b *TransferBatch) RecordHandoff(handoff CustodyHandoff, at time.Time) (Event, error) {
	if err := b.EnsureMutable(); err != nil {
		return Event{}, err
	}
	if b.Status != StatusCustodyInTransit {
		return Event{}, ErrInvalidTransition
	}
	issues := []ValidationIssue{}
	RequireText(handoff.ID, "id", &issues)
	RequireText(handoff.ReleasedBy, "releasedBy", &issues)
	RequireText(handoff.ReceivedBy, "receivedBy", &issues)
	RequireText(handoff.Location, "location", &issues)
	RequireText(handoff.SealCondition, "sealCondition", &issues)
	RequireText(handoff.TemperatureSummary, "temperatureSummary", &issues)
	if handoff.ReleasedBy != "" && handoff.ReleasedBy == handoff.ReceivedBy {
		issues = append(issues, NewIssue("same_party", "receivedBy", "交出方与接收方不能相同"))
	}
	if handoff.OccurredAt.IsZero() {
		issues = append(issues, NewIssue("required", "occurredAt", "交接时间不能为空"))
	}
	if !handoff.OccurredAt.IsZero() && handoff.OccurredAt.Before(b.UpdatedAt) {
		issues = append(issues, NewIssue("time_order", "occurredAt", "交接时间不能早于离场核验"))
	}
	expected := len(b.Handoffs) + 1
	if handoff.Sequence != 0 && handoff.Sequence != expected {
		issues = append(issues, NewIssue("sequence", "sequence", "交接序号必须连续"))
	}
	for _, previous := range b.Handoffs {
		if previous.Status == HandoffOpen {
			issues = append(issues, NewIssue("handoff_open", "status", "存在尚未确认的交接"))
		}
		if !handoff.OccurredAt.After(previous.OccurredAt) {
			issues = append(issues, NewIssue("time_order", "occurredAt", "交接时间必须晚于此前交接"))
		}
	}
	if len(b.Handoffs) > 0 {
		previous := b.Handoffs[len(b.Handoffs)-1]
		if previous.Status == HandoffConfirmed && handoff.ReleasedBy != "" && handoff.ReleasedBy != previous.ReceivedBy {
			issues = append(issues, ValidationIssue{
				Code: "responsibility_chain", Field: "releasedBy",
				Expected: previous.ReceivedBy, Actual: handoff.ReleasedBy,
				Message: "本次交出方必须等于上一段已确认交接的接收方",
			})
		}
	}
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	handoff.BatchID, handoff.Sequence, handoff.Status = b.ID, expected, HandoffConfirmed
	return NewEvent(EventHandoffRecorded, b.ID, b.Version+1, at, handoff)
}

func (b *TransferBatch) RecalculateTransitOverview() {
	b.TransitOverview = TransitOverview{}
	for _, handoff := range b.Handoffs {
		if handoff.Status != HandoffConfirmed {
			continue
		}
		b.TransitOverview.CurrentCustodian = handoff.ReceivedBy
		b.TransitOverview.LastHandoffLocation = handoff.Location
		occurredAt := handoff.OccurredAt
		b.TransitOverview.LastHandoffAt = &occurredAt
		if handoff.SealCondition != "intact" {
			b.TransitOverview.SealAnomalyCount++
		}
		if handoff.TemperatureSummary == "" {
			b.TransitOverview.TemperatureAnomalyCount++
		}
	}
}
