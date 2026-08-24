package persistence

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
)

func (s *Store) loadLog() error {
	file, err := os.Open(s.logPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("打开事件日志: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			var record LogRecord
			if err := json.Unmarshal(line, &record); err != nil {
				return fmt.Errorf("事件日志第 %d 行无法解析: %w", s.sequence+1, err)
			}
			if err := s.replayRecord(record); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return fmt.Errorf("读取事件日志: %w", readErr)
		}
	}
	return nil
}

func (s *Store) replayRecord(record LogRecord) error {
	expectedSequence := s.sequence + 1
	if record.SchemaVersion != SchemaVersion {
		return fmt.Errorf("事件 %d schemaVersion 不受支持", expectedSequence)
	}
	if record.Sequence != expectedSequence {
		return fmt.Errorf("事件序号不连续: 期望 %d，实际 %d", expectedSequence, record.Sequence)
	}
	if record.PreviousDigest != s.lastDigest {
		return fmt.Errorf("事件 %d 前置校验摘要不匹配", record.Sequence)
	}
	canonical, err := record.canonical()
	if err != nil {
		return fmt.Errorf("事件 %d 规范化失败: %w", record.Sequence, err)
	}
	if !audit.Verify(record.PreviousDigest, record.Sequence, canonical, record.Digest) {
		return fmt.Errorf("事件 %d 校验摘要损坏", record.Sequence)
	}
	if !record.Audit.Complete() {
		return fmt.Errorf("事件 %d 审计字段不完整", record.Sequence)
	}
	batch := s.batches[record.Event.BatchID]
	if batch == nil {
		batch = &domain.TransferBatch{}
	}
	if err := batch.Apply(record.Event); err != nil {
		return fmt.Errorf("重放事件 %d: %w", record.Sequence, err)
	}
	s.batches[record.Event.BatchID] = batch
	s.records = append(s.records, record)
	s.idempotency[record.IdempotencyKey] = IdempotencyResult{RequestDigest: record.RequestDigest, Response: append(json.RawMessage(nil), record.Response...)}
	s.sequence, s.lastDigest = record.Sequence, record.Digest
	return nil
}
