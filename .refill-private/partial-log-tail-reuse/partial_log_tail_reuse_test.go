package partial_log_tail_reuse_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/persistence"
)

func TestRecoveredPartialTailIsRemovedBeforeNextCommit(t *testing.T) {
	dir := t.TempDir()
	store, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("open initial store: %v", err)
	}
	service := application.NewService(store)
	created, err := service.CreateBatch(application.CreateBatchCommand{
		CommandMeta: application.CommandMeta{
			ExpectedVersion: 0,
			IdempotencyKey:  "partial-tail-create",
			Actor:           "collector-a",
			Role:            application.RoleCollector,
			RequestID:       "request-create",
		},
		BatchCode:             "PARTIAL-TAIL-001",
		CollectionSite:        "恢复测试样地",
		DestinationRepository: "恢复测试保藏库",
		LeadCollector:         "采集负责人",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	logPath := filepath.Join(dir, "events.jsonl")
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		t.Fatalf("open event log for crash residue: %v", err)
	}
	if _, err := file.Write([]byte(`{"schemaVersion":1,"sequence":2`)); err != nil {
		file.Close()
		t.Fatalf("write simulated partial tail: %v", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		t.Fatalf("sync simulated partial tail: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close simulated partial tail: %v", err)
	}

	recovered, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("first restart should recover the uncommitted tail: %v", err)
	}
	recoveredService := application.NewService(recovered)
	now := time.Now().UTC()
	if _, err := recoveredService.RegisterPermit(created.Batch.ID, application.RegisterPermitCommand{
		CommandMeta: application.CommandMeta{
			ExpectedVersion: 1,
			IdempotencyKey:  "partial-tail-permit",
			Actor:           "collector-a",
			Role:            application.RoleCollector,
			RequestID:       "request-permit",
		},
		PermitNumber:         "PERMIT-PARTIAL-TAIL",
		ValidFrom:            now.Add(-time.Hour),
		ValidUntil:           now.Add(time.Hour),
		AllowedMaterialCodes: []string{"PLANT"},
		QuantityLimit:        10,
		Issuer:               "保护地管理机构",
	}); err != nil {
		t.Fatalf("commit after recovery: %v", err)
	}

	reopened, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("second restart must retain the post-recovery commit: %v", err)
	}
	batch, err := reopened.Get(created.Batch.ID)
	if err != nil {
		t.Fatalf("get batch after second restart: %v", err)
	}
	if batch.Version != 2 || len(batch.Permits) != 1 {
		t.Fatalf("post-recovery commit was not replayed: version=%d permits=%d", batch.Version, len(batch.Permits))
	}
}
