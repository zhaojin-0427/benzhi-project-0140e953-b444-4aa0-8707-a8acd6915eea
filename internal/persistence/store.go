package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"specimen-custody-gate/internal/domain"
)

type IdempotencyResult struct {
	RequestDigest string          `json:"requestDigest"`
	Response      json.RawMessage `json:"response"`
}

type Store struct {
	mu           sync.RWMutex
	dir          string
	logPath      string
	snapshotPath string
	batches      map[string]*domain.TransferBatch
	records      []LogRecord
	idempotency  map[string]IdempotencyResult
	lastDigest   string
	sequence     uint64
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("存储目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("创建存储目录: %w", err)
	}
	s := &Store{dir: dir, logPath: filepath.Join(dir, "events.jsonl"), snapshotPath: filepath.Join(dir, "projection.json"), batches: map[string]*domain.TransferBatch{}, idempotency: map[string]IdempotencyResult{}}
	if err := s.loadLog(); err != nil {
		return nil, err
	}
	stale, err := s.validateSnapshot()
	if err != nil {
		return nil, err
	}
	if stale {
		if err := s.writeSnapshot(); err != nil {
			return nil, fmt.Errorf("刷新滞后投影快照: %w", err)
		}
	}
	return s, nil
}

func (s *Store) Get(batchID string) (*domain.TransferBatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	batch, ok := s.batches[batchID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneBatch(batch)
}

func (s *Store) List() ([]domain.TransferBatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.TransferBatch, 0, len(s.batches))
	for _, item := range s.batches {
		copy, err := cloneBatch(item)
		if err != nil {
			return nil, err
		}
		result = append(result, *copy)
	}
	return result, nil
}

func cloneBatch(batch *domain.TransferBatch) (*domain.TransferBatch, error) {
	data, err := json.Marshal(batch)
	if err != nil {
		return nil, err
	}
	var result domain.TransferBatch
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Store) LookupIdempotency(key, requestDigest string) (json.RawMessage, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, ok := s.idempotency[key]
	if !ok {
		return nil, false, nil
	}
	if result.RequestDigest != requestDigest {
		return nil, false, domain.ErrIdempotencyConflict
	}
	return append(json.RawMessage(nil), result.Response...), true, nil
}

func (s *Store) RecordsForBatch(batchID string) []LogRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []LogRecord{}
	for _, record := range s.records {
		if record.Event.BatchID == batchID {
			result = append(result, record)
		}
	}
	return result
}
