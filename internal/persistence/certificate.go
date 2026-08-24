package persistence

import (
	"encoding/json"
	"fmt"

	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
)

type CertificateIssuanceInspection struct {
	Valid  bool
	Reason string
}

func (s *Store) InspectCertificateIssuance(batchID string, certificate domain.DepositCertificate) CertificateIssuanceInspection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for index, record := range s.records {
		if record.Event.BatchID != batchID || record.Event.Type != domain.EventCertificateIssued {
			continue
		}
		var issued domain.DepositCertificate
		if err := json.Unmarshal(record.Event.Data, &issued); err != nil {
			return CertificateIssuanceInspection{Reason: "certificate.issued 事件数据无法解析"}
		}
		if issued.ID != certificate.ID {
			continue
		}
		previous := ""
		if index > 0 {
			previous = s.records[index-1].Digest
		}
		canonical, err := record.canonical()
		if err != nil || record.PreviousDigest != previous || !audit.Verify(previous, record.Sequence, canonical, record.Digest) {
			return CertificateIssuanceInspection{Reason: "签发事件校验链摘要不连续"}
		}
		if record.Event.Version != certificate.BatchVersion || record.Audit.BatchVersion != certificate.BatchVersion {
			return CertificateIssuanceInspection{Reason: "签发事件版本与凭证不一致"}
		}
		if record.Audit.BatchID != batchID || record.Audit.Result != "accepted" || record.Audit.Action != "approve_deposit" {
			return CertificateIssuanceInspection{Reason: "签发事件审计结果与凭证不一致"}
		}
		if issued != certificate {
			return CertificateIssuanceInspection{Reason: "签发事件中的凭证字段与当前投影不一致"}
		}
		return CertificateIssuanceInspection{Valid: true}
	}
	return CertificateIssuanceInspection{Reason: fmt.Sprintf("未找到凭证 %s 对应的 certificate.issued 事件", certificate.ID)}
}
