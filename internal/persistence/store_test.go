package persistence

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
)

func TestCommitReopenAndIdempotency(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	event, err := domain.CreateBatch(domain.CreateBatchInput{ID: "b", BatchCode: "B", CollectionSite: "地点", DestinationRepository: "库", LeadCollector: "人"}, now)
	if err != nil {
		t.Fatal(err)
	}
	response := json.RawMessage(`{"ok":true}`)
	record := audit.Build(audit.Context{Actor: "actor", Role: "collector", RequestID: "request", IdempotencyKey: "key"}, "b", "create_batch", "accepted", 1, now)
	stored, replay, err := store.Commit(CommitRequest{ExpectedVersion: 0, Event: event, Audit: record, IdempotencyKey: "key", RequestDigest: "request-digest", Response: response})
	if err != nil || replay || string(stored) != string(response) {
		t.Fatalf("首次提交异常: replay=%v response=%s err=%v", replay, stored, err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := reopened.Get("b")
	if err != nil || batch.Version != 1 {
		t.Fatalf("重启恢复异常: batch=%+v err=%v", batch, err)
	}
	stored, replay, err = reopened.Commit(CommitRequest{ExpectedVersion: 0, Event: event, Audit: record, IdempotencyKey: "key", RequestDigest: "request-digest", Response: response})
	if err != nil || !replay || string(stored) != string(response) {
		t.Fatalf("幂等重放异常: replay=%v response=%s err=%v", replay, stored, err)
	}
}

func TestOpenLocatesTamperedRecord(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	event, _ := domain.CreateBatch(domain.CreateBatchInput{ID: "b", BatchCode: "B", CollectionSite: "地点", DestinationRepository: "库", LeadCollector: "人"}, now)
	record := audit.Build(audit.Context{Actor: "actor", Role: "collector", RequestID: "request", IdempotencyKey: "key"}, "b", "create_batch", "accepted", 1, now)
	if _, _, err := store.Commit(CommitRequest{ExpectedVersion: 0, Event: event, Audit: record, IdempotencyKey: "key", RequestDigest: "digest", Response: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.logPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["digest"] = "tampered"
	damaged, _ := json.Marshal(raw)
	damaged = append(damaged, '\n')
	if err := os.WriteFile(store.logPath, damaged, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("篡改后的日志应无法恢复")
	}
}
