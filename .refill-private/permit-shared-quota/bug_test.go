package permit_shared_quota_test

import (
	"errors"
	"testing"
	"time"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/domain"
	"specimen-custody-gate/internal/persistence"
)

func TestPermitQuotaIsSharedAcrossMaterials(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	created, err := service.CreateBatch(application.CreateBatchCommand{
		CommandMeta: meta(0, "quota-create"), BatchCode: "QUOTA-1",
		CollectionSite: "样地", DestinationRepository: "保藏库", LeadCollector: "负责人",
	})
	if err != nil {
		t.Fatal(err)
	}
	batchID := created.Batch.ID
	_, err = service.RegisterPermit(batchID, application.RegisterPermitCommand{
		CommandMeta: meta(1, "quota-permit"), PermitNumber: "PERMIT-1",
		ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour),
		AllowedMaterialCodes: []string{"PLANT", "ANIMAL"}, QuantityLimit: 3, Issuer: "管理机构",
	})
	if err != nil {
		t.Fatal(err)
	}
	registerSpecimen(t, service, batchID, 2, "quota-plant", "PLANT", "container-1", "seal-1", now)
	registerSpecimen(t, service, batchID, 3, "quota-animal", "ANIMAL", "container-2", "seal-2", now)

	_, err = service.VerifyDeparture(batchID, meta(4, "quota-departure"))
	var field *domain.FieldError
	if !errors.As(err, &field) || !hasQuantityLimit(field.Issues) {
		t.Fatalf("TestPermitQuotaIsSharedAcrossMaterials: 单个许可总量 3 被两个材料各占用 2 后仍通过离场核验: %v", err)
	}
	batch, getErr := store.Get(batchID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if batch.Status != domain.StatusDraft || batch.Version != 4 {
		t.Fatalf("额度核验失败不应推进状态: %s/%d", batch.Status, batch.Version)
	}
}

func meta(version int, key string) application.CommandMeta {
	return application.CommandMeta{
		ExpectedVersion: version, IdempotencyKey: key,
		Actor: "采集员", Role: application.RoleCollector, RequestID: "request-" + key,
	}
}

func registerSpecimen(t *testing.T, service *application.Service, batchID string, version int, key, material, container, seal string, at time.Time) {
	t.Helper()
	_, err := service.RegisterSpecimen(batchID, application.RegisterSpecimenCommand{
		CommandMeta: meta(version, key), MaterialCode: material, SourceDescription: "来源",
		CollectedAt: at, ContainerCode: container, SealCode: seal,
		PreservationRequirement: "cold", Quantity: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func hasQuantityLimit(issues []domain.ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Code == "quantity_limit" && issue.ShortfallQuantity == 1 {
			return true
		}
	}
	return false
}
