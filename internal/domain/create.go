package domain

import "time"

type CreateBatchInput struct {
	ID                    string
	BatchCode             string
	CollectionSite        string
	DestinationRepository string
	LeadCollector         string
}

func CreateBatch(input CreateBatchInput, at time.Time) (Event, error) {
	issues := []ValidationIssue{}
	RequireText(input.ID, "id", &issues)
	RequireText(input.BatchCode, "batchCode", &issues)
	RequireText(input.CollectionSite, "collectionSite", &issues)
	RequireText(input.DestinationRepository, "destinationRepository", &issues)
	RequireText(input.LeadCollector, "leadCollector", &issues)
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	data := BatchCreatedData{BatchCode: input.BatchCode, CollectionSite: input.CollectionSite, DestinationRepository: input.DestinationRepository, LeadCollector: input.LeadCollector}
	return NewEvent(EventBatchCreated, input.ID, 1, at, data)
}
