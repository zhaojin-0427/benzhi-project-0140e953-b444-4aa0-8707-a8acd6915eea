package domain

import (
	"encoding/json"
	"fmt"
)

func NewBatchFromEvents(events []Event) (*TransferBatch, error) {
	if len(events) == 0 {
		return nil, ErrNotFound
	}
	b := &TransferBatch{}
	for _, event := range events {
		if err := b.Apply(event); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (b *TransferBatch) Apply(event Event) error {
	if b.Version != 0 && event.Version != b.Version+1 {
		return fmt.Errorf("事件版本不连续: 当前 %d，收到 %d", b.Version, event.Version)
	}
	switch event.Type {
	case EventBatchCreated:
		var data BatchCreatedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		b.ID, b.BatchCode, b.CollectionSite = event.BatchID, data.BatchCode, data.CollectionSite
		b.DestinationRepository, b.LeadCollector = data.DestinationRepository, data.LeadCollector
		b.Status, b.CreatedAt = StatusDraft, event.OccurredAt
	case EventPermitRegistered:
		var data CollectionPermit
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		b.Permits = append(b.Permits, data)
		b.RecalculatePermitQuota()
	case EventSpecimenRegistered:
		var data Specimen
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		b.Specimens = append(b.Specimens, data)
		b.RecalculatePermitQuota()
	case EventDepartureVerified:
		b.Status = StatusCustodyInTransit
	case EventHandoffRecorded:
		var data CustodyHandoff
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		b.Handoffs = append(b.Handoffs, data)
		b.RecalculateTransitOverview()
	case EventArrivalInspected:
		var data ArrivalInspectedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		b.Discrepancies = data.Discrepancies
		b.RecalculateDiscrepancyActions()
		if len(data.Discrepancies) == 0 {
			b.Status = StatusReviewPending
		} else {
			b.Status = StatusRemediationRequired
		}
	case EventRemediationSubmitted:
		var data RemediationSubmittedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		for i := range b.Discrepancies {
			if b.Discrepancies[i].ID == data.DiscrepancyID {
				b.Discrepancies[i].RemediationNote = data.Note
				b.Discrepancies[i].EvidenceDigest = data.EvidenceDigest
				b.Discrepancies[i].Status = DiscrepancyRemediated
				b.Discrepancies[i].Revisions = append(b.Discrepancies[i].Revisions, RemediationRevision{
					Revision: data.Revision, SubmittedBy: data.SubmittedBy, Note: data.Note,
					EvidenceDigest: data.EvidenceDigest, SubmittedAt: event.OccurredAt,
				})
			}
		}
		b.RecalculateDiscrepancyActions()
	case EventDiscrepancyReviewed:
		var data DiscrepancyReviewedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		for i := range b.Discrepancies {
			if b.Discrepancies[i].ID == data.DiscrepancyID {
				b.Discrepancies[i].ReviewedBy = data.ReviewedBy
				b.Discrepancies[i].ReviewedAt = &event.OccurredAt
				if data.Approved {
					b.Discrepancies[i].Status = DiscrepancyClosed
				} else {
					b.Discrepancies[i].Status = DiscrepancyRejected
				}
				for revisionIndex := range b.Discrepancies[i].Revisions {
					revision := &b.Discrepancies[i].Revisions[revisionIndex]
					if revision.Revision == data.Revision {
						revision.Review = &RemediationReview{ReviewedBy: data.ReviewedBy, Approved: data.Approved, Opinion: data.Opinion, ReviewedAt: event.OccurredAt}
					}
				}
			}
		}
		b.RecalculateDiscrepancyActions()
	case EventArrivalReverified:
		b.Status = StatusReviewPending
	case EventCertificateIssued:
		var data DepositCertificate
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		b.Certificate, b.ManifestDigest, b.Status = &data, data.ManifestDigest, StatusDeposited
	default:
		return fmt.Errorf("未知领域事件类型 %q", event.Type)
	}
	b.Version, b.UpdatedAt = event.Version, event.OccurredAt
	return nil
}

func (b *TransferBatch) EnsureMutable() error {
	if b.Status == StatusDeposited || b.Certificate != nil {
		return ErrFrozen
	}
	return nil
}
