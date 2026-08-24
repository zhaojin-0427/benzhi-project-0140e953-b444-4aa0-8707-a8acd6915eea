package domain

import (
	"errors"
	"testing"
	"time"
)

func TestPermitQuotaUsesStableEffectiveAllocation(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	batch := newTestBatch(t, now)
	for _, permit := range []CollectionPermit{
		{ID: "p1", PermitNumber: "P-1", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), AllowedMaterialCodes: []string{"PLANT"}, QuantityLimit: 2, Issuer: "机构"},
		{ID: "p2", PermitNumber: "P-2", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), AllowedMaterialCodes: []string{"PLANT"}, QuantityLimit: 3, Issuer: "机构"},
	} {
		event, err := batch.RegisterPermit(permit, now)
		applyTestEvent(t, batch, event, err)
	}
	event, err := batch.RegisterSpecimen(Specimen{ID: "s1", MaterialCode: "PLANT", SourceDescription: "来源", CollectedAt: now, ContainerCode: "c1", SealCode: "seal1", PreservationRequirement: "cold", Quantity: 4}, now)
	applyTestEvent(t, batch, event, err)
	if len(batch.PermitQuota) != 2 || batch.PermitQuota[0].UsedQuantity != 2 || batch.PermitQuota[1].UsedQuantity != 2 || batch.PermitQuota[1].Remaining != 1 {
		t.Fatalf("许可额度稳定分配异常: %+v", batch.PermitQuota)
	}
	if len(batch.PermitWarnings) != 0 || !batch.DepartureReadiness().Ready {
		t.Fatalf("额度充足不应产生预警: %+v", batch.PermitWarnings)
	}
}

func TestPermitQuotaWarningContainsSpecimenAndShortfall(t *testing.T) {
	now := time.Now().UTC()
	batch := newTestBatch(t, now)
	event, err := batch.RegisterPermit(CollectionPermit{ID: "p", PermitNumber: "P", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), AllowedMaterialCodes: []string{"PLANT"}, QuantityLimit: 1, Issuer: "机构"}, now)
	applyTestEvent(t, batch, event, err)
	event, err = batch.RegisterSpecimen(Specimen{ID: "s", MaterialCode: "PLANT", SourceDescription: "来源", CollectedAt: now, ContainerCode: "c", SealCode: "seal", PreservationRequirement: "cold", Quantity: 3}, now)
	applyTestEvent(t, batch, event, err)
	if len(batch.PermitWarnings) != 1 || batch.PermitWarnings[0].SpecimenID != "s" || batch.PermitWarnings[0].ShortfallQuantity != 2 {
		t.Fatalf("许可缺口预警异常: %+v", batch.PermitWarnings)
	}
}

func TestDepartureReadinessIsStableAndReadOnly(t *testing.T) {
	batch := &TransferBatch{ID: "b", Status: StatusDraft, Version: 8, Specimens: []Specimen{
		{ID: "s2", MaterialCode: "UNKNOWN", ContainerCode: "same", SealCode: "same", Quantity: 1},
		{ID: "s1", MaterialCode: "UNKNOWN", ContainerCode: "same", SealCode: "same", Quantity: 1},
	}}
	beforeStatus, beforeVersion := batch.Status, batch.Version
	report := batch.DepartureReadiness()
	if report.Ready || report.Counts.Permit == 0 || report.Counts.Identification == 0 || report.Counts.Preservation == 0 {
		t.Fatalf("预检分类异常: %+v", report)
	}
	if batch.Status != beforeStatus || batch.Version != beforeVersion {
		t.Fatalf("预检改变了状态或版本: %s/%d", batch.Status, batch.Version)
	}
	for i := 1; i < len(report.Issues); i++ {
		previous, current := report.Issues[i-1], report.Issues[i]
		if previous.Code > current.Code || previous.Code == current.Code && previous.Field > current.Field {
			t.Fatalf("问题清单未稳定排序: %+v", report.Issues)
		}
	}
}

func TestHandoffResponsibilityContinuityAndOverview(t *testing.T) {
	now := time.Now().UTC()
	batch := &TransferBatch{ID: "b", Status: StatusCustodyInTransit, Version: 1, UpdatedAt: now}
	event, err := batch.RecordHandoff(CustodyHandoff{ID: "h1", ReleasedBy: "甲", ReceivedBy: "乙", OccurredAt: now.Add(time.Minute), Location: "出口", SealCondition: "intact", TemperatureSummary: "2-6C"}, now)
	applyTestEvent(t, batch, event, err)
	_, err = batch.RecordHandoff(CustodyHandoff{ID: "h2", ReleasedBy: "丁", ReceivedBy: "丙", OccurredAt: now.Add(2 * time.Minute), Location: "中转", SealCondition: "intact", TemperatureSummary: "2-6C"}, now)
	var field *FieldError
	if !errors.As(err, &field) || field.Issues[len(field.Issues)-1].Code != "responsibility_chain" {
		t.Fatalf("期望责任链断裂字段错误，实际 %v", err)
	}
	if batch.Version != 2 || batch.TransitOverview.CurrentCustodian != "乙" {
		t.Fatalf("失败交接不应改变投影: %+v", batch.TransitOverview)
	}
	event, err = batch.RecordHandoff(CustodyHandoff{ID: "h2", ReleasedBy: "乙", ReceivedBy: "丙", OccurredAt: now.Add(2 * time.Minute), Location: "中转", SealCondition: "broken", TemperatureSummary: "2-6C"}, now)
	applyTestEvent(t, batch, event, err)
	if batch.TransitOverview.CurrentCustodian != "丙" || batch.TransitOverview.SealAnomalyCount != 1 {
		t.Fatalf("在途概览异常: %+v", batch.TransitOverview)
	}
}

func TestRemediationRevisionHistoryAndReviewerSeparation(t *testing.T) {
	now := time.Now().UTC()
	batch := &TransferBatch{ID: "b", Status: StatusRemediationRequired, Version: 1, Discrepancies: []Discrepancy{{ID: "d", Status: DiscrepancyOpen}}}
	event, err := batch.SubmitRemediationBy("d", "第一版", "e1", "张三", now)
	applyTestEvent(t, batch, event, err)
	_, err = batch.ReviewRevision("d", 1, "张三", false, "自审", now)
	var field *FieldError
	if !errors.As(err, &field) || field.Issues[0].Code != "reviewer_conflict" {
		t.Fatalf("期望操作者分离错误，实际 %v", err)
	}
	event, err = batch.ReviewRevision("d", 1, "李四", false, "证据不足", now.Add(time.Minute))
	applyTestEvent(t, batch, event, err)
	event, err = batch.SubmitRemediationBy("d", "第二版", "e2", "张三", now.Add(2*time.Minute))
	applyTestEvent(t, batch, event, err)
	_, err = batch.ReviewRevision("d", 1, "李四", true, "过期版本", now.Add(3*time.Minute))
	if !errors.As(err, &field) || field.Issues[0].Code != "stale_revision" {
		t.Fatalf("期望过期修订错误，实际 %v", err)
	}
	event, err = batch.ReviewRevision("d", 2, "李四", true, "复验通过", now.Add(3*time.Minute))
	applyTestEvent(t, batch, event, err)
	item := batch.Discrepancies[0]
	if item.Status != DiscrepancyClosed || len(item.Revisions) != 2 || item.Revisions[0].Review.Opinion != "证据不足" || item.Revisions[1].Review.Opinion != "复验通过" {
		t.Fatalf("整改与复验历史异常: %+v", item)
	}
}

func TestDiscrepancyFilterKeepsFullBatchSummary(t *testing.T) {
	batch := &TransferBatch{Discrepancies: []Discrepancy{
		{ID: "d1", SpecimenID: "s1", Category: "seal", Status: DiscrepancyOpen},
		{ID: "d2", SpecimenID: "s2", Category: "quantity", Status: DiscrepancyRemediated},
		{ID: "d3", SpecimenID: "s3", Category: "preservation", Status: DiscrepancyClosed},
	}}
	result := batch.QueryDiscrepancies(DiscrepancyFilter{Status: "open", Category: "seal"})
	if result.MatchedCount != 1 || result.Discrepancies[0].ID != "d1" {
		t.Fatalf("组合筛选异常: %+v", result)
	}
	if result.Summary.ByStatus["open"] != 1 || result.Summary.ByStatus["remediated"] != 1 || result.Summary.ByStatus["closed"] != 1 || result.Summary.ByCategory["preservation"] != 1 {
		t.Fatalf("全批次摘要异常: %+v", result.Summary)
	}
}
