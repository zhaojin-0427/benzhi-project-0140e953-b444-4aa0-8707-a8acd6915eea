package domain

var discrepancyCategories = []string{"missing", "container", "seal", "preservation", "quantity", "unexpected"}

type DiscrepancyFilter struct {
	Status     string
	Category   string
	SpecimenID string
}

type DiscrepancySummary struct {
	ByStatus   map[string]int `json:"byStatus"`
	ByCategory map[string]int `json:"byCategory"`
}

type DiscrepancyQueryResult struct {
	Discrepancies []Discrepancy      `json:"discrepancies"`
	Summary       DiscrepancySummary `json:"summary"`
	MatchedCount  int                `json:"matchedCount"`
}

func KnownDiscrepancyStatus(value string) bool {
	switch DiscrepancyStatus(value) {
	case DiscrepancyOpen, DiscrepancyRemediated, DiscrepancyClosed, DiscrepancyRejected:
		return true
	default:
		return false
	}
}

func KnownDiscrepancyCategory(value string) bool {
	for _, category := range discrepancyCategories {
		if value == category {
			return true
		}
	}
	return false
}

func (b *TransferBatch) QueryDiscrepancies(filter DiscrepancyFilter) DiscrepancyQueryResult {
	summary := DiscrepancySummary{
		ByStatus: map[string]int{
			string(DiscrepancyOpen): 0, string(DiscrepancyRemediated): 0,
			string(DiscrepancyClosed): 0, string(DiscrepancyRejected): 0,
		},
		ByCategory: map[string]int{},
	}
	for _, category := range discrepancyCategories {
		summary.ByCategory[category] = 0
	}
	matched := make([]Discrepancy, 0)
	for _, item := range b.Discrepancies {
		summary.ByStatus[string(item.Status)]++
		summary.ByCategory[item.Category]++
		if filter.Status != "" && string(item.Status) != filter.Status {
			continue
		}
		if filter.Category != "" && item.Category != filter.Category {
			continue
		}
		if filter.SpecimenID != "" && item.SpecimenID != filter.SpecimenID {
			continue
		}
		matched = append(matched, item)
	}
	return DiscrepancyQueryResult{Discrepancies: matched, Summary: summary, MatchedCount: len(matched)}
}
