package shared_data_dir_corruption_test

import (
	"encoding/json"
	"testing"
	"time"

	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
	"specimen-custody-gate/internal/persistence"
)

type openResult struct {
	store *persistence.Store
	err   error
}

func TestTwoStoresCannotCorruptSharedEventLog(t *testing.T) {
	dir := t.TempDir()
	start := make(chan struct{})
	opened := make(chan openResult, 2)
	for range 2 {
		go func() {
			<-start
			store, err := persistence.Open(dir)
			opened <- openResult{store: store, err: err}
		}()
	}
	close(start)
	first, second := <-opened, <-opened
	if first.err != nil || second.err != nil {
		return // 独占打开并明确拒绝第二个写入者也是正确行为。
	}

	commitBatch(t, first.store, "batch-a", "BATCH-A", "key-a", "digest-a")
	commitBatch(t, second.store, "batch-b", "BATCH-B", "key-b", "digest-b")
	if _, err := persistence.Open(dir); err != nil {
		t.Fatalf("TestTwoStoresCannotCorruptSharedEventLog: 两个写入者从相同序号追加后，数据目录已无法重放: %v", err)
	}
}

func commitBatch(t *testing.T, store *persistence.Store, id, code, key, digest string) {
	t.Helper()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	event, err := domain.CreateBatch(domain.CreateBatchInput{
		ID: id, BatchCode: code, CollectionSite: "样地",
		DestinationRepository: "保藏库", LeadCollector: "负责人",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	record := audit.Build(audit.Context{
		Actor: "采集员", Role: "collector", RequestID: "request-" + id,
		IdempotencyKey: key,
	}, id, "create_batch", "accepted", 1, now)
	_, _, err = store.Commit(persistence.CommitRequest{
		ExpectedVersion: 0, Event: event, Audit: record,
		IdempotencyKey: key, RequestDigest: digest, Response: json.RawMessage(`{"ok":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
}
