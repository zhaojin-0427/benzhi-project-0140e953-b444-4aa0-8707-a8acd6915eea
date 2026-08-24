package domain

import (
	"errors"
	"testing"
	"time"
)

func applyTestEvent(t *testing.T, batch *TransferBatch, event Event, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("生成事件失败: %v", err)
	}
	if err := batch.Apply(event); err != nil {
		t.Fatalf("应用事件失败: %v", err)
	}
}

func newTestBatch(t *testing.T, now time.Time) *TransferBatch {
	t.Helper()
	batch := &TransferBatch{}
	event, err := CreateBatch(CreateBatchInput{ID: "batch-1", BatchCode: "B-1", CollectionSite: "样地", DestinationRepository: "保藏库", LeadCollector: "负责人"}, now)
	applyTestEvent(t, batch, event, err)
	return batch
}

func TestDepartureRejectsPermitScopeAndThenPasses(t *testing.T) {
	now := time.Now().UTC()
	batch := newTestBatch(t, now)
	permit := CollectionPermit{ID: "permit-1", PermitNumber: "P-1", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), AllowedMaterialCodes: []string{"PLANT"}, QuantityLimit: 2, Issuer: "管理机构"}
	event, err := batch.RegisterPermit(permit, now)
	applyTestEvent(t, batch, event, err)
	specimen := Specimen{ID: "s-1", MaterialCode: "ANIMAL", SourceDescription: "来源", CollectedAt: now, ContainerCode: "c-1", SealCode: "seal-1", PreservationRequirement: "cold", Quantity: 1}
	event, err = batch.RegisterSpecimen(specimen, now)
	applyTestEvent(t, batch, event, err)
	_, err = batch.VerifyDeparture(now)
	var field *FieldError
	if !errors.As(err, &field) {
		t.Fatalf("期望结构化许可范围问题，实际 %v", err)
	}
	if field.Issues[0].Code != "permit_scope" {
		t.Fatalf("问题代码异常: %+v", field.Issues)
	}
}

func TestRemediationMustCloseBeforeCertificateAndFreeze(t *testing.T) {
	now := time.Now().UTC()
	batch := newTestBatch(t, now)
	event, err := batch.RegisterPermit(CollectionPermit{ID: "p", PermitNumber: "P", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), AllowedMaterialCodes: []string{"PLANT"}, QuantityLimit: 5, Issuer: "机构"}, now)
	applyTestEvent(t, batch, event, err)
	event, err = batch.RegisterSpecimen(Specimen{ID: "s", MaterialCode: "PLANT", SourceDescription: "来源", CollectedAt: now, ContainerCode: "c", SealCode: "seal", PreservationRequirement: "cold", Quantity: 2}, now)
	applyTestEvent(t, batch, event, err)
	event, err = batch.VerifyDeparture(now)
	applyTestEvent(t, batch, event, err)
	event, err = batch.RecordHandoff(CustodyHandoff{ID: "h", ReleasedBy: "甲", ReceivedBy: "乙", OccurredAt: now.Add(time.Minute), Location: "出口", SealCondition: "intact", TemperatureSummary: "2-6C"}, now)
	applyTestEvent(t, batch, event, err)
	event, err = batch.InspectArrival([]ReceivedSpecimen{{SpecimenID: "s", ContainerCode: "c", SealCode: "seal", SealCondition: "intact", PreservationCondition: "cold", Quantity: 1}}, now, func() string { return "issue" })
	applyTestEvent(t, batch, event, err)
	if _, err := batch.IssueCertificate("cert", "审核人", now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("整改前不应签发: %v", err)
	}
	event, err = batch.SubmitRemediation("issue", "补齐样本", "digest", now)
	applyTestEvent(t, batch, event, err)
	event, err = batch.ReviewDiscrepancy("issue", "验收员", true, now)
	applyTestEvent(t, batch, event, err)
	event, err = batch.ReverifyArrival(now)
	applyTestEvent(t, batch, event, err)
	event, err = batch.IssueCertificate("cert", "审核人", now)
	applyTestEvent(t, batch, event, err)
	if !VerifyCertificate(*batch.Certificate) {
		t.Fatal("凭证摘要应可验证")
	}
	if _, err := batch.RegisterSpecimen(Specimen{}, now); !errors.Is(err, ErrFrozen) {
		t.Fatalf("冻结后修改应被拒绝: %v", err)
	}
}

func TestHandoffRejectsReverseTime(t *testing.T) {
	now := time.Now().UTC()
	batch := &TransferBatch{ID: "b", Status: StatusCustodyInTransit, Version: 5, Handoffs: []CustodyHandoff{{ID: "h1", OccurredAt: now, Status: HandoffConfirmed}}}
	_, err := batch.RecordHandoff(CustodyHandoff{ID: "h2", ReleasedBy: "甲", ReceivedBy: "乙", OccurredAt: now.Add(-time.Minute), Location: "地点", SealCondition: "intact", TemperatureSummary: "cold"}, now)
	var field *FieldError
	if !errors.As(err, &field) {
		t.Fatalf("期望交接时序字段错误，实际 %v", err)
	}
}
