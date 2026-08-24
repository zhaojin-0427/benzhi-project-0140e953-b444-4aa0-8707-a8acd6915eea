package application

import "specimen-custody-gate/internal/domain"

func (s *Service) DepartureReadiness(batchID string) (*domain.DepartureReadinessReport, error) {
	batch, err := s.store.Get(batchID)
	if err != nil {
		return nil, err
	}
	report := batch.DepartureReadiness()
	return &report, nil
}
