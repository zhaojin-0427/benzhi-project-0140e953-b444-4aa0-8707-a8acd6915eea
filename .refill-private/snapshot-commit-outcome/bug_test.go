package snapshot_commit_outcome_test

import (
	"os"
	"path/filepath"
	"testing"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/persistence"
)

func TestSnapshotFailureCannotReturnErrorAfterCommit(t *testing.T) {
	dir := t.TempDir()
	store, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "projection.json"), 0o750); err != nil {
		t.Fatal(err)
	}

	service := application.NewService(store)
	_, commitErr := service.CreateBatch(application.CreateBatchCommand{
		CommandMeta: application.CommandMeta{
			ExpectedVersion: 0,
			IdempotencyKey:  "snapshot-failure-create",
			Actor:           "采集员",
			Role:            application.RoleCollector,
			RequestID:       "snapshot-failure-request",
		},
		BatchCode:             "SNAPSHOT-FAILURE",
		CollectionSite:        "样地",
		DestinationRepository: "保藏库",
		LeadCollector:         "负责人",
	})

	batches, listErr := store.List()
	if listErr != nil {
		t.Fatal(listErr)
	}
	if commitErr != nil && len(batches) != 0 {
		t.Fatalf("TestSnapshotFailureCannotReturnErrorAfterCommit: 调用方收到失败，但事件和内存投影已经提交: %v", commitErr)
	}
	if commitErr == nil && len(batches) != 1 {
		t.Fatalf("成功返回后应存在一个批次，实际 %d", len(batches))
	}
}
