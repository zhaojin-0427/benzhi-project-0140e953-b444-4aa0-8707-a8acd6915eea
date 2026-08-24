package application

import "specimen-custody-gate/internal/domain"

type readinessCacheEntry struct {
	version int
	report  *domain.DepartureReadinessReport
}

func (s *Service) DepartureReadiness(batchID string) (*domain.DepartureReadinessReport, error) {
	batch, err := s.store.Get(batchID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.readiness[batchID]; ok && cached.version == batch.Version {
		return cached.report, nil
	}
	report := batch.DepartureReadiness()
	s.readiness[batchID] = readinessCacheEntry{version: batch.Version, report: &report}
	return &report, nil
}
