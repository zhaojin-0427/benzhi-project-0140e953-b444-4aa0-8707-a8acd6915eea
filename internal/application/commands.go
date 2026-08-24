package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"specimen-custody-gate/internal/domain"
)

func (s *Service) CreateBatch(command CreateBatchCommand) (*BatchResult, error) {
	return s.CreateBatchContext(context.Background(), command)
}

func (s *Service) CreateBatchContext(ctx context.Context, command CreateBatchCommand) (*BatchResult, error) {
	if err := requireRole(command.Role, RoleCollector); err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(command.IdempotencyKey))
	batchID := "batch-" + hex.EncodeToString(hash[:12])
	return s.executeContext(ctx, batchID, "create_batch", command.CommandMeta, command, func(_ *domain.TransferBatch) (domain.Event, error) {
		return domain.CreateBatch(domain.CreateBatchInput{ID: batchID, BatchCode: command.BatchCode, CollectionSite: command.CollectionSite, DestinationRepository: command.DestinationRepository, LeadCollector: command.LeadCollector}, s.now())
	})
}

func (s *Service) RegisterPermit(batchID string, command RegisterPermitCommand) (*BatchResult, error) {
	if err := requireRole(command.Role, RoleCollector); err != nil {
		return nil, err
	}
	return s.execute(batchID, "register_permit", command.CommandMeta, command, func(batch *domain.TransferBatch) (domain.Event, error) {
		permit := domain.CollectionPermit{ID: s.id("permit"), PermitNumber: command.PermitNumber, ValidFrom: command.ValidFrom, ValidUntil: command.ValidUntil, AllowedMaterialCodes: command.AllowedMaterialCodes, QuantityLimit: command.QuantityLimit, Issuer: command.Issuer}
		return batch.RegisterPermit(permit, s.now())
	})
}

func (s *Service) RegisterSpecimen(batchID string, command RegisterSpecimenCommand) (*BatchResult, error) {
	if err := requireRole(command.Role, RoleCollector); err != nil {
		return nil, err
	}
	return s.execute(batchID, "register_specimen", command.CommandMeta, command, func(batch *domain.TransferBatch) (domain.Event, error) {
		specimen := domain.Specimen{ID: s.id("specimen"), MaterialCode: command.MaterialCode, SourceDescription: command.SourceDescription, CollectedAt: command.CollectedAt, ContainerCode: command.ContainerCode, SealCode: command.SealCode, PreservationRequirement: command.PreservationRequirement, Quantity: command.Quantity}
		return batch.RegisterSpecimen(specimen, s.now())
	})
}

func (s *Service) VerifyDeparture(batchID string, meta CommandMeta) (*BatchResult, error) {
	if err := requireRole(meta.Role, RoleCollector); err != nil {
		return nil, err
	}
	return s.execute(batchID, "verify_departure", meta, meta, func(batch *domain.TransferBatch) (domain.Event, error) { return batch.VerifyDeparture(s.now()) })
}

func (s *Service) RecordHandoff(batchID string, command HandoffCommand) (*BatchResult, error) {
	if err := requireRole(command.Role, RoleCustodian); err != nil {
		return nil, err
	}
	return s.execute(batchID, "record_handoff", command.CommandMeta, command, func(batch *domain.TransferBatch) (domain.Event, error) {
		handoff := domain.CustodyHandoff{ID: s.id("handoff"), Sequence: command.Sequence, ReleasedBy: command.ReleasedBy, ReceivedBy: command.ReceivedBy, OccurredAt: command.OccurredAt, Location: command.Location, SealCondition: command.SealCondition, TemperatureSummary: command.TemperatureSummary}
		return batch.RecordHandoff(handoff, s.now())
	})
}

func (s *Service) InspectArrival(batchID string, command ArrivalCommand) (*BatchResult, error) {
	if err := requireRole(command.Role, RoleReceiver); err != nil {
		return nil, err
	}
	return s.execute(batchID, "inspect_arrival", command.CommandMeta, command, func(batch *domain.TransferBatch) (domain.Event, error) {
		return batch.InspectArrival(command.Received, s.now(), func() string { return s.id("issue") })
	})
}
