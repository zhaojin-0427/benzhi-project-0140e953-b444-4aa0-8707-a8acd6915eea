package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
)

type CommitRequest struct {
	ExpectedVersion int
	Event           domain.Event
	Audit           audit.Record
	IdempotencyKey  string
	RequestDigest   string
	Response        json.RawMessage
}

func (s *Store) Commit(request CommitRequest) (json.RawMessage, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.idempotency[request.IdempotencyKey]; ok {
		if existing.RequestDigest != request.RequestDigest {
			return nil, false, domain.ErrIdempotencyConflict
		}
		return append(json.RawMessage(nil), existing.Response...), true, nil
	}
	current := s.batches[request.Event.BatchID]
	version := 0
	if current != nil {
		version = current.Version
	}
	if version != request.ExpectedVersion {
		return nil, false, domain.ErrVersionConflict
	}
	if request.Event.Version != version+1 {
		return nil, false, domain.ErrVersionConflict
	}
	if request.IdempotencyKey == "" || request.RequestDigest == "" {
		return nil, false, errors.New("提交缺少幂等信息")
	}
	if !request.Audit.Complete() {
		return nil, false, errors.New("提交缺少审计信息")
	}
	candidate := &domain.TransferBatch{}
	if current != nil {
		var err error
		candidate, err = cloneBatch(current)
		if err != nil {
			return nil, false, err
		}
	}
	if err := candidate.Apply(request.Event); err != nil {
		return nil, false, err
	}
	if request.Event.Type == domain.EventBatchCreated {
		for id, existing := range s.batches {
			if id != candidate.ID && existing.BatchCode == candidate.BatchCode {
				return nil, false, &domain.FieldError{Issues: []domain.ValidationIssue{domain.NewIssue("duplicate", "batchCode", "批次编号已经存在")}}
			}
		}
	}
	record := LogRecord{SchemaVersion: SchemaVersion, Sequence: s.sequence + 1, PreviousDigest: s.lastDigest, Event: request.Event, Audit: request.Audit, IdempotencyKey: request.IdempotencyKey, RequestDigest: request.RequestDigest, Response: append(json.RawMessage(nil), request.Response...)}
	canonical, err := record.canonical()
	if err != nil {
		return nil, false, err
	}
	record.Digest = audit.Digest(record.PreviousDigest, record.Sequence, canonical)
	line, err := json.Marshal(record)
	if err != nil {
		return nil, false, err
	}
	file, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, false, fmt.Errorf("打开事件日志: %w", err)
	}
	if _, err = file.Write(append(line, '\n')); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return nil, false, fmt.Errorf("同步事件日志: %w", err)
	}
	if closeErr != nil {
		return nil, false, fmt.Errorf("关闭事件日志: %w", closeErr)
	}
	s.batches[request.Event.BatchID] = candidate
	s.records = append(s.records, record)
	s.idempotency[request.IdempotencyKey] = IdempotencyResult{RequestDigest: request.RequestDigest, Response: append(json.RawMessage(nil), request.Response...)}
	s.sequence, s.lastDigest = record.Sequence, record.Digest
	// Once the event log has been durably synced, the commit is irreversible.
	// The projection snapshot is only a recovery cache derived from the event
	// log, so a failure while refreshing it must not turn an already-committed
	// result into an error: otherwise callers would observe a failed response
	// while queries and idempotent retries see the committed state. Keep the
	// accepted result; the stale snapshot is rebuilt on the next successful
	// commit or on reopen via validateSnapshot.
	if err := s.writeSnapshot(); err != nil {
		return append(json.RawMessage(nil), request.Response...), false, nil
	}
	return append(json.RawMessage(nil), request.Response...), false, nil
}
