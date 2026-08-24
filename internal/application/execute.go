package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
	"specimen-custody-gate/internal/persistence"
)

type eventBuilder func(batch *domain.TransferBatch) (domain.Event, error)

func requestDigest(action, batchID string, meta CommandMeta, command any) (string, error) {
	data, err := json.Marshal(struct {
		Action  string `json:"action"`
		BatchID string `json:"batchId"`
		Actor   string `json:"actor"`
		Role    string `json:"role"`
		Command any    `json:"command"`
	}{Action: action, BatchID: batchID, Actor: meta.Actor, Role: meta.Role, Command: command})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func (s *Service) execute(batchID, action string, meta CommandMeta, command any, build eventBuilder) (*BatchResult, error) {
	if err := validateMeta(meta); err != nil {
		return nil, err
	}
	digest, err := requestDigest(action, batchID, meta, command)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if response, found, err := s.store.LookupIdempotency(meta.IdempotencyKey, digest); err != nil {
		return nil, err
	} else if found {
		var result BatchResult
		if err := json.Unmarshal(response, &result); err != nil {
			return nil, fmt.Errorf("解析幂等响应: %w", err)
		}
		result.Replay = true
		return &result, nil
	}
	var batch *domain.TransferBatch
	if action == "create_batch" {
		batch = &domain.TransferBatch{}
	} else {
		batch, err = s.store.Get(batchID)
		if err != nil {
			return nil, err
		}
	}
	if batch.Version != meta.ExpectedVersion {
		return nil, domain.ErrVersionConflict
	}
	event, err := build(batch)
	if err != nil {
		return nil, err
	}
	candidate, err := cloneAndApply(batch, event)
	if err != nil {
		return nil, err
	}
	result := &BatchResult{Batch: candidate, EventType: event.Type}
	response, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	record := audit.Build(audit.Context{Actor: meta.Actor, Role: meta.Role, RequestID: meta.RequestID, IdempotencyKey: meta.IdempotencyKey}, batchID, action, "accepted", event.Version, event.OccurredAt)
	stored, replay, err := s.store.Commit(persistence.CommitRequest{ExpectedVersion: meta.ExpectedVersion, Event: event, Audit: record, IdempotencyKey: meta.IdempotencyKey, RequestDigest: digest, Response: response})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := json.Unmarshal(stored, result); err != nil {
			return nil, err
		}
		result.Replay = true
	}
	return result, nil
}

func cloneAndApply(batch *domain.TransferBatch, event domain.Event) (*domain.TransferBatch, error) {
	data, err := json.Marshal(batch)
	if err != nil {
		return nil, err
	}
	var result domain.TransferBatch
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if err := result.Apply(event); err != nil {
		return nil, err
	}
	return &result, nil
}
