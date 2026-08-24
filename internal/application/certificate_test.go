package application

import (
	"encoding/json"
	"testing"
	"time"

	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
	"specimen-custody-gate/internal/persistence"
)

func TestCertificateDeepVerificationAfterRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	created, err := domain.CreateBatch(domain.CreateBatchInput{ID: "b", BatchCode: "B", CollectionSite: "地点", DestinationRepository: "库", LeadCollector: "人"}, now)
	if err != nil {
		t.Fatal(err)
	}
	createAudit := audit.Build(audit.Context{Actor: "采集员", Role: RoleCollector, RequestID: "r1", IdempotencyKey: "k1"}, "b", "create_batch", "accepted", 1, now)
	if _, _, err := store.Commit(persistence.CommitRequest{ExpectedVersion: 0, Event: created, Audit: createAudit, IdempotencyKey: "k1", RequestDigest: "d1", Response: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	batch, err := store.Get("b")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := batch.ManifestDigestValue()
	if err != nil {
		t.Fatal(err)
	}
	certificate := domain.DepositCertificate{ID: "cert", BatchID: "b", BatchVersion: 2, ManifestDigest: manifest, SpecimenCount: 0, ApprovedBy: "合规员", IssuedAt: now.Add(time.Minute)}
	certificate.VerificationDigest = domain.CertificateVerificationDigest(certificate)
	issued, err := domain.NewEvent(domain.EventCertificateIssued, "b", 2, certificate.IssuedAt, certificate)
	if err != nil {
		t.Fatal(err)
	}
	issueAudit := audit.Build(audit.Context{Actor: "合规员", Role: RoleCompliance, RequestID: "r2", IdempotencyKey: "k2"}, "b", "approve_deposit", "accepted", 2, certificate.IssuedAt)
	if _, _, err := store.Commit(persistence.CommitRequest{ExpectedVersion: 1, Event: issued, Audit: issueAudit, IdempotencyKey: "k2", RequestDigest: "d2", Response: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	reopened, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	verification, err := NewService(reopened).VerifyCertificate("b")
	if err != nil {
		t.Fatal(err)
	}
	if !verification.OverallValid || !verification.VerificationDigest.Valid || !verification.ManifestDigest.Valid || !verification.QuantityAndVersion.Valid || !verification.IssuanceEvent.Valid {
		t.Fatalf("重启恢复后的凭证深度校验失败: %+v", verification)
	}
}
