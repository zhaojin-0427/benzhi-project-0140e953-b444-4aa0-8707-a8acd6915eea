package timeline_cache_cross_service_test

import (
	"testing"
	"time"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/domain"
	"specimen-custody-gate/internal/persistence"
)

func TestTimelineCacheObservesCommitsFromPeerService(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reader := application.NewService(store)
	writer := application.NewService(store)

	created, err := writer.CreateBatch(application.CreateBatchCommand{
		CommandMeta: application.CommandMeta{
			ExpectedVersion: 0,
			IdempotencyKey:  "create-key",
			Actor:           "采集负责人",
			Role:            application.RoleCollector,
			RequestID:       "create-request",
		},
		BatchCode:             "CACHE-001",
		CollectionSite:        "东部样地",
		DestinationRepository: "科研保藏库",
		LeadCollector:         "采集负责人",
	})
	if err != nil {
		t.Fatal(err)
	}
	batchID := created.Batch.ID

	before, err := reader.Timeline(batchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].EventType != domain.EventBatchCreated {
		t.Fatalf("首次时间线异常: %+v", before)
	}

	_, err = writer.RegisterPermit(batchID, application.RegisterPermitCommand{
		CommandMeta: application.CommandMeta{
			ExpectedVersion: 1,
			IdempotencyKey:  "permit-key",
			Actor:           "采集负责人",
			Role:            application.RoleCollector,
			RequestID:       "permit-request",
		},
		PermitNumber:         "PERMIT-001",
		ValidFrom:            time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		ValidUntil:           time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		AllowedMaterialCodes: []string{"leaf"},
		QuantityLimit:        10,
		Issuer:               "保护地管理局",
	})
	if err != nil {
		t.Fatal(err)
	}

	after, err := reader.Timeline(batchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[1].EventType != domain.EventPermitRegistered {
		t.Fatalf("另一个 Service 已提交 permit.registered 后时间线仍为旧视图: %+v", after)
	}
}
