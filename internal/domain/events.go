package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	EventBatchCreated         = "batch.created"
	EventPermitRegistered     = "permit.registered"
	EventSpecimenRegistered   = "specimen.registered"
	EventDepartureVerified    = "departure.verified"
	EventHandoffRecorded      = "handoff.recorded"
	EventArrivalInspected     = "arrival.inspected"
	EventRemediationSubmitted = "remediation.submitted"
	EventDiscrepancyReviewed  = "discrepancy.reviewed"
	EventArrivalReverified    = "arrival.reverified"
	EventCertificateIssued    = "certificate.issued"
)

type Event struct {
	Type       string          `json:"type"`
	BatchID    string          `json:"batchId"`
	Version    int             `json:"version"`
	OccurredAt time.Time       `json:"occurredAt"`
	Data       json.RawMessage `json:"data"`
}

func NewEvent(kind, batchID string, version int, at time.Time, value any) (Event, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return Event{}, fmt.Errorf("编码领域事件: %w", err)
	}
	return Event{Type: kind, BatchID: batchID, Version: version, OccurredAt: at.UTC(), Data: b}, nil
}

type BatchCreatedData struct {
	BatchCode             string `json:"batchCode"`
	CollectionSite        string `json:"collectionSite"`
	DestinationRepository string `json:"destinationRepository"`
	LeadCollector         string `json:"leadCollector"`
}

type DepartureVerifiedData struct {
	IssuesChecked int `json:"issuesChecked"`
}

type ArrivalInspectedData struct {
	Discrepancies []Discrepancy `json:"discrepancies"`
}

type RemediationSubmittedData struct {
	DiscrepancyID  string `json:"discrepancyId"`
	Revision       int    `json:"revision"`
	SubmittedBy    string `json:"submittedBy"`
	Note           string `json:"note"`
	EvidenceDigest string `json:"evidenceDigest"`
}

type DiscrepancyReviewedData struct {
	DiscrepancyID string `json:"discrepancyId"`
	Revision      int    `json:"revision"`
	Approved      bool   `json:"approved"`
	ReviewedBy    string `json:"reviewedBy"`
	Opinion       string `json:"opinion"`
}
