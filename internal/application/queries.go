package application

import "specimen-custody-gate/internal/domain"

type readinessCacheEntry struct {
	version int
	report  domain.DepartureReadinessReport
}

func (s *Service) DepartureReadiness(batchID string) (*domain.DepartureReadinessReport, error) {
	batch, err := s.store.Get(batchID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.readiness[batchID]; ok && cached.version == batch.Version {
		report := cloneDepartureReadinessReport(cached.report)
		return &report, nil
	}
	computed := batch.DepartureReadiness()
	s.readiness[batchID] = readinessCacheEntry{version: batch.Version, report: cloneDepartureReadinessReport(computed)}
	return &computed, nil
}

// cloneDepartureReadinessReport returns a copy of report whose Issues slice
// does not share backing storage with the input, so callers mutating the
// returned Issues cannot corrupt the cached value or vice versa.
func cloneDepartureReadinessReport(report domain.DepartureReadinessReport) domain.DepartureReadinessReport {
	clone := report
	if report.Issues != nil {
		issues := make([]domain.ValidationIssue, len(report.Issues))
		copy(issues, report.Issues)
		clone.Issues = issues
	}
	return clone
}
