package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

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

// lockPathFor returns the path of the cross-process lock file guarding dir.
func lockPathFor(dir string) string { return filepath.Join(dir, ".store.lock") }

// acquireLock opens the lock file in dir and acquires an exclusive flock(2)
// that serializes Store instances across separate processes sharing the same
// data directory. The returned release func releases the lock and closes the
// handle. The lock file is created with mode 0o600 and never holds event data.
func acquireLock(dir string) (*os.File, error) {
	path := lockPathFor(dir)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开存储锁文件: %w", err)
	}
	if err := flock(file, syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("锁定存储目录: %w", err)
	}
	return file, nil
}

func releaseLock(file *os.File) error {
	if file == nil {
		return nil
	}
	_ = flock(file, syscall.LOCK_UN)
	return file.Close()
}

// flock wraps syscall.Flock for any *os.File.
func flock(file *os.File, how int) error {
	return syscall.Flock(int(file.Fd()), how)
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("存储目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("创建存储目录: %w", err)
	}
	// Hold the cross-process lock for the whole open path so concurrent
	// writers cannot mutate the log while we validate the digest chain and
	// refresh the projection snapshot.
	lockFile, err := acquireLock(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = releaseLock(lockFile) }()
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
