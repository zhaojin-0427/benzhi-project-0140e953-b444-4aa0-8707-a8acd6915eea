package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type snapshot struct {
	SchemaVersion int                          `json:"schemaVersion"`
	LastSequence  uint64                       `json:"lastSequence"`
	LastDigest    string                       `json:"lastDigest"`
	Batches       map[string]*json.RawMessage  `json:"batches"`
	Idempotency   map[string]IdempotencyResult `json:"idempotency"`
}

func (s *Store) writeSnapshot() error {
	encoded := make(map[string]*json.RawMessage, len(s.batches))
	for id, batch := range s.batches {
		data, err := json.Marshal(batch)
		if err != nil {
			return err
		}
		raw := json.RawMessage(data)
		encoded[id] = &raw
	}
	data, err := json.MarshalIndent(snapshot{SchemaVersion: SchemaVersion, LastSequence: s.sequence, LastDigest: s.lastDigest, Batches: encoded, Idempotency: s.idempotency}, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(s.dir, ".projection-*.tmp")
	if err != nil {
		return fmt.Errorf("创建候选快照: %w", err)
	}
	name := temp.Name()
	cleanup := func() { _ = os.Remove(name) }
	if err := temp.Chmod(0o640); err != nil {
		temp.Close()
		cleanup()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		cleanup()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		cleanup()
		return err
	}
	if err := temp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(name, s.snapshotPath); err != nil {
		cleanup()
		return fmt.Errorf("原子替换快照: %w", err)
	}
	dir, err := os.Open(filepath.Dir(s.snapshotPath))
	if err != nil {
		return err
	}
	err = dir.Sync()
	closeErr := dir.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func (s *Store) validateSnapshot() (bool, error) {
	data, err := os.ReadFile(s.snapshotPath)
	if errors.Is(err, os.ErrNotExist) {
		return s.sequence > 0, nil
	}
	if err != nil {
		return false, fmt.Errorf("读取投影快照: %w", err)
	}
	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return false, fmt.Errorf("解析投影快照: %w", err)
	}
	if snap.SchemaVersion != SchemaVersion {
		return false, errors.New("投影快照 schemaVersion 不受支持")
	}
	if snap.LastSequence > s.sequence {
		return false, errors.New("投影快照序号超前于事件日志")
	}
	if snap.LastSequence == 0 && snap.LastDigest != "" {
		return false, errors.New("空投影快照含有非法校验摘要")
	}
	if snap.LastSequence > 0 {
		record := s.records[snap.LastSequence-1]
		if record.Digest != snap.LastDigest {
			return false, fmt.Errorf("投影快照在事件 %d 处与校验链不一致", snap.LastSequence)
		}
	}
	return snap.LastSequence < s.sequence, nil
}
