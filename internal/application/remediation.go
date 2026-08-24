package application

import (
	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
)

func (s *Service) SubmitRemediation(batchID, discrepancyID string, command RemediationCommand) (*BatchResult, error) {
	if err := requireRole(command.Role, RoleCollector, RoleCustodian); err != nil {
		return nil, err
	}
	return s.execute(batchID, "submit_remediation", command.CommandMeta, command, func(batch *domain.TransferBatch) (domain.Event, error) {
		return batch.SubmitRemediationBy(discrepancyID, command.RemediationNote, command.EvidenceDigest, command.Actor, s.now())
	})
}

func (s *Service) ReviewDiscrepancy(batchID, discrepancyID string, command ReviewCommand) (*BatchResult, error) {
	if err := requireRole(command.Role, RoleReceiver); err != nil {
		return nil, err
	}
	return s.execute(batchID, "review_discrepancy", command.CommandMeta, command, func(batch *domain.TransferBatch) (domain.Event, error) {
		return batch.ReviewRevision(discrepancyID, command.Revision, command.Actor, command.Approved, command.Opinion, s.now())
	})
}

func (s *Service) ReverifyArrival(batchID string, meta CommandMeta) (*BatchResult, error) {
	if err := requireRole(meta.Role, RoleReceiver); err != nil {
		return nil, err
	}
	return s.execute(batchID, "reverify_arrival", meta, meta, func(batch *domain.TransferBatch) (domain.Event, error) { return batch.ReverifyArrival(s.now()) })
}

func (s *Service) ApproveDeposit(batchID string, command ApproveCommand) (*BatchResult, error) {
	if err := requireRole(command.Role, RoleCompliance); err != nil {
		return nil, err
	}
	return s.execute(batchID, "approve_deposit", command.CommandMeta, command, func(batch *domain.TransferBatch) (domain.Event, error) {
		return batch.IssueCertificate(s.id("certificate"), command.ApprovedBy, s.now())
	})
}

func (s *Service) ListDiscrepancies(batchID string, filter domain.DiscrepancyFilter) (*domain.DiscrepancyQueryResult, error) {
	batch, err := s.store.Get(batchID)
	if err != nil {
		return nil, err
	}
	result := batch.QueryDiscrepancies(filter)
	return &result, nil
}

func (s *Service) VerifyCertificate(batchID string) (*CertificateVerification, error) {
	batch, err := s.store.Get(batchID)
	if err != nil {
		return nil, err
	}
	if batch.Certificate == nil {
		return nil, domain.ErrNotFound
	}
	certificate := *batch.Certificate
	verificationDigest := VerificationCheck{Valid: domain.VerifyCertificate(certificate)}
	if !verificationDigest.Valid {
		verificationDigest.Reason = "凭证 verificationDigest 复算不一致"
	}
	manifestValue, manifestErr := batch.ManifestDigestValue()
	manifestDigest := VerificationCheck{Valid: manifestErr == nil && manifestValue == certificate.ManifestDigest && batch.ManifestDigest == certificate.ManifestDigest}
	if manifestErr != nil {
		manifestDigest.Reason = "冻结样本清单摘要无法复算"
	} else if !manifestDigest.Valid {
		manifestDigest.Reason = "当前冻结样本清单、批次投影或凭证的 manifestDigest 不一致"
	}
	quantityAndVersion := VerificationCheck{Valid: certificate.BatchID == batch.ID && certificate.SpecimenCount == len(batch.Specimens) && certificate.BatchVersion == batch.Version}
	if !quantityAndVersion.Valid {
		quantityAndVersion.Reason = "凭证批次编号、样本数量或签发版本与冻结批次不一致"
	}
	issuance := s.store.InspectCertificateIssuance(batchID, certificate)
	issuanceEvent := VerificationCheck{Valid: issuance.Valid, Reason: issuance.Reason}
	overall := verificationDigest.Valid && manifestDigest.Valid && quantityAndVersion.Valid && issuanceEvent.Valid
	return &CertificateVerification{
		Certificate: batch.Certificate, OverallValid: overall, Valid: overall,
		VerificationDigest: verificationDigest, ManifestDigest: manifestDigest,
		QuantityAndVersion: quantityAndVersion, IssuanceEvent: issuanceEvent,
	}, nil
}

func (s *Service) Timeline(batchID string) ([]audit.TimelineEntry, error) {
	if _, err := s.store.Get(batchID); err != nil {
		return nil, err
	}
	records := s.store.RecordsForBatch(batchID)
	entries := make([]audit.TimelineEntry, 0, len(records))
	for _, record := range records {
		entries = append(entries, audit.TimelineEntry{Sequence: record.Sequence, EventType: record.Event.Type, BatchVersion: record.Event.Version, Actor: record.Audit.Actor, Role: record.Audit.Role, Action: record.Audit.Action, Result: record.Audit.Result, OccurredAt: record.Event.OccurredAt.Format("2006-01-02T15:04:05.999999999Z07:00"), PreviousDigest: record.PreviousDigest, Digest: record.Digest})
	}
	audit.SortTimeline(entries)
	return entries, nil
}
