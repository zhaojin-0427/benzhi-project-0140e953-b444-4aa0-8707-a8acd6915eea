package failed_commit_projection_alias_test

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

func TestFailedCommitDoesNotMutateProjection(t *testing.T) {
	dir := t.TempDir()
	store, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("打开存储: %v", err)
	}

	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	created := mustEvent(t, domain.EventBatchCreated, "batch-alias", 1, now, domain.BatchCreatedData{
		BatchCode:             "ALIAS-001",
		CollectionSite:        "核心保护区样地",
		DestinationRepository: "国家标本馆",
		LeadCollector:         "采集负责人",
	})
	commitEvent(t, store, created, 0, "create-key")

	inspected := mustEvent(t, domain.EventArrivalInspected, "batch-alias", 2, now.Add(time.Minute), domain.ArrivalInspectedData{
		Discrepancies: []domain.Discrepancy{{
			ID:          "issue-1",
			BatchID:     "batch-alias",
			Category:    "seal",
			Description: "封签状态不符",
			Status:      domain.DiscrepancyOpen,
		}},
	})
	commitEvent(t, store, inspected, 1, "arrival-key")

	logPath := filepath.Join(dir, "events.jsonl")
	if err := os.Remove(logPath); err != nil {
		t.Fatalf("移除事件日志以模拟资源失效: %v", err)
	}
	if err := os.Mkdir(logPath, 0o750); err != nil {
		t.Fatalf("将日志路径替换为目录: %v", err)
	}

	remediation := mustEvent(t, domain.EventRemediationSubmitted, "batch-alias", 3, now.Add(2*time.Minute), domain.RemediationSubmittedData{
		DiscrepancyID:  "issue-1",
		Revision:       1,
		SubmittedBy:    "采集负责人",
		Note:           "已重新封签",
		EvidenceDigest: "evidence-001",
	})
	_, _, err = store.Commit(commitRequest(remediation, 2, "remediation-key"))
	if err == nil {
		t.Fatal("日志资源失效时提交应返回错误")
	}

	after, err := store.Get("batch-alias")
	if err != nil {
		t.Fatalf("读取失败提交后的投影: %v", err)
	}
	if after.Version != 2 {
		t.Fatalf("失败提交后版本被推进: got %d want 2", after.Version)
	}
	if got := after.Discrepancies[0].Status; got != domain.DiscrepancyOpen {
		t.Fatalf("失败提交污染了投影中的差异状态: got %q want %q", got, domain.DiscrepancyOpen)
	}
	if got := len(after.Discrepancies[0].Revisions); got != 0 {
		t.Fatalf("失败提交泄漏了整改修订: got %d want 0", got)
	}
}

func mustEvent(t *testing.T, kind, batchID string, version int, at time.Time, value any) domain.Event {
	t.Helper()
	event, err := domain.NewEvent(kind, batchID, version, at, value)
	if err != nil {
		t.Fatalf("构造事件 %s: %v", kind, err)
	}
	return event
}

func commitEvent(t *testing.T, store *persistence.Store, event domain.Event, expectedVersion int, key string) {
	t.Helper()
	if _, _, err := store.Commit(commitRequest(event, expectedVersion, key)); err != nil {
		t.Fatalf("提交准备事件 %s: %v", event.Type, err)
	}
}

func commitRequest(event domain.Event, expectedVersion int, key string) persistence.CommitRequest {
	record := audit.Build(audit.Context{
		Actor:          "私有复现",
		Role:           "collector",
		RequestID:      "request-" + key,
		IdempotencyKey: key,
	}, event.BatchID, event.Type, "accepted", event.Version, event.OccurredAt)
	return persistence.CommitRequest{
		ExpectedVersion: expectedVersion,
		Event:           event,
		Audit:           record,
		IdempotencyKey:  key,
		RequestDigest:   "digest-" + key,
		Response:        json.RawMessage(`{"ok":true}`),
	}
}
