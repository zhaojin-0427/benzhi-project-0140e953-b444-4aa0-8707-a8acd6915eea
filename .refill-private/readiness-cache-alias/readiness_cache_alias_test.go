package readinesscachealias_test

import (
	"testing"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/persistence"
)

func TestReadinessCacheDoesNotExposeMutableState(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	created, err := service.CreateBatch(application.CreateBatchCommand{
		CommandMeta: application.CommandMeta{
			ExpectedVersion: 0,
			IdempotencyKey:  "readiness-cache-create",
			Actor:           "采集负责人",
			Role:            application.RoleCollector,
			RequestID:       "readiness-cache-request",
		},
		BatchCode:             "READINESS-CACHE-001",
		CollectionSite:        "缓存隔离样区",
		DestinationRepository: "缓存隔离保藏库",
		LeadCollector:         "采集负责人",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.DepartureReadiness(created.Batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	issueIndex := -1
	for index := range first.Issues {
		if first.Issues[index].Code == "permit_missing" {
			issueIndex = index
			break
		}
	}
	if issueIndex < 0 {
		t.Fatal("预检结果缺少 permit_missing")
	}
	original := first.Issues[issueIndex].Message
	first.Issues[issueIndex].Message = "调用方伪造的缓存内容"

	second, err := service.DepartureReadiness(created.Batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Issues[issueIndex].Message != original {
		t.Fatalf("TestReadinessCacheDoesNotExposeMutableState: 第二次查询被首次调用方污染，得到 %q", second.Issues[issueIndex].Message)
	}
}
