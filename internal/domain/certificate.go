package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"
)

type manifestItem struct {
	ID           string `json:"id"`
	MaterialCode string `json:"materialCode"`
	Container    string `json:"container"`
	Seal         string `json:"seal"`
	Quantity     int    `json:"quantity"`
}

func (b *TransferBatch) ManifestDigestValue() (string, error) {
	items := make([]manifestItem, 0, len(b.Specimens))
	for _, specimen := range b.Specimens {
		items = append(items, manifestItem{ID: specimen.ID, MaterialCode: specimen.MaterialCode, Container: specimen.ContainerCode, Seal: specimen.SealCode, Quantity: specimen.Quantity})
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func CertificateVerificationDigest(c DepositCertificate) string {
	value := c.ID + "|" + c.BatchID + "|" + strconv.Itoa(c.BatchVersion) + "|" + c.ManifestDigest + "|" + strconv.Itoa(c.SpecimenCount) + "|" + c.ApprovedBy + "|" + c.IssuedAt.UTC().Format(time.RFC3339Nano)
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func VerifyCertificate(c DepositCertificate) bool {
	return c.VerificationDigest != "" && c.VerificationDigest == CertificateVerificationDigest(c)
}

func (b *TransferBatch) IssueCertificate(id, approvedBy string, at time.Time) (Event, error) {
	if err := b.EnsureMutable(); err != nil {
		return Event{}, err
	}
	if b.Status != StatusReviewPending {
		return Event{}, ErrInvalidTransition
	}
	issues := b.ValidateDeparture()
	RequireText(id, "id", &issues)
	RequireText(approvedBy, "approvedBy", &issues)
	if len(b.Handoffs) == 0 {
		issues = append(issues, NewIssue("handoff_missing", "handoffs", "缺少保管交接记录"))
	}
	for _, item := range b.Discrepancies {
		if item.Status != DiscrepancyClosed {
			issues = append(issues, NewIssue("discrepancy_open", "discrepancies", "存在未关闭问题"))
		}
	}
	if len(issues) > 0 {
		return Event{}, &FieldError{Issues: issues}
	}
	digest, err := b.ManifestDigestValue()
	if err != nil {
		return Event{}, err
	}
	certificate := DepositCertificate{ID: id, BatchID: b.ID, BatchVersion: b.Version + 1, ManifestDigest: digest, SpecimenCount: len(b.Specimens), ApprovedBy: approvedBy, IssuedAt: at.UTC()}
	certificate.VerificationDigest = CertificateVerificationDigest(certificate)
	return NewEvent(EventCertificateIssued, b.ID, b.Version+1, at, certificate)
}
