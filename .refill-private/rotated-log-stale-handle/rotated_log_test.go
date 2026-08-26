package rotated_log_stale_handle_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
	"specimen-custody-gate/internal/persistence"
)

func TestRotatedEventLogReceivesLaterCommits(t *testing.T) {
	dir := t.TempDir()
	store, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 25, 9, 0, 0, 0, time.UTC)
	created, err := domain.CreateBatch(domain.CreateBatchInput{
		ID: "batch-rotation", BatchCode: "ROT-001", CollectionSite: "样地一",
		DestinationRepository: "保藏库", LeadCollector: "采集员甲",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	commitEvent(t, store, 0, created, "create-key", "create-digest", now)

	logPath := filepath.Join(dir, "events.jsonl")
	beforeRotation, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(logPath, filepath.Join(dir, "events.jsonl.1")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, beforeRotation, 0o640); err != nil {
		t.Fatal(err)
	}

	permit := domain.CollectionPermit{
		ID: "permit-rotation", BatchID: "batch-rotation", PermitNumber: "P-001",
		ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour),
		AllowedMaterialCodes: []string{"leaf"}, QuantityLimit: 2, Issuer: "许可机关",
	}
	registered, err := domain.NewEvent(domain.EventPermitRegistered, "batch-rotation", 2, now.Add(time.Minute), permit)
	if err != nil {
		t.Fatal(err)
	}
	commitEvent(t, store, 1, registered, "permit-key", "permit-digest", now.Add(time.Minute))

	reopened, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("事件日志轮转后的已确认提交必须能够恢复: %v", err)
	}
	batch, err := reopened.Get("batch-rotation")
	if err != nil {
		t.Fatal(err)
	}
	if batch.Version != 2 || len(batch.Permits) != 1 {
		t.Fatalf("轮转后的提交未从当前事件日志恢复: version=%d permits=%d", batch.Version, len(batch.Permits))
	}
}

func commitEvent(t *testing.T, store *persistence.Store, expectedVersion int, event domain.Event, key, digest string, at time.Time) {
	t.Helper()
	record := audit.Build(audit.Context{
		Actor: "测试操作者", Role: "collector", RequestID: "request-" + key, IdempotencyKey: key,
	}, event.BatchID, event.Type, "accepted", event.Version, at)
	_, replay, err := store.Commit(persistence.CommitRequest{
		ExpectedVersion: expectedVersion, Event: event, Audit: record,
		IdempotencyKey: key, RequestDigest: digest, Response: json.RawMessage(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("提交 %s 失败: %v", event.Type, err)
	}
	if replay {
		t.Fatalf("提交 %s 被错误识别为重放", event.Type)
	}
}
