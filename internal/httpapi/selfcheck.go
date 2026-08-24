package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"specimen-custody-gate/internal/application"
)

type selfcheckClient struct {
	baseURL string
	client  *http.Client
	serial  int
}

func RunSelfcheck(ctx context.Context, address string) error {
	c := &selfcheckClient{baseURL: "http://" + address, client: &http.Client{Timeout: 3 * time.Second}}
	now := time.Now().UTC().Truncate(time.Second)
	create := map[string]any{"expectedVersion": 0, "idempotencyKey": "selfcheck-create", "batchCode": "SC-001", "collectionSite": "自检保护地样区", "destinationRepository": "自检保藏库", "leadCollector": "采集负责人"}
	var result application.BatchResult
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches", "selfcheck-collector", application.RoleCollector, create, http.StatusCreated, &result); err != nil {
		return err
	}
	batchID := result.Batch.ID
	var replay application.BatchResult
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches", "selfcheck-collector", application.RoleCollector, create, http.StatusCreated, &replay); err != nil {
		return fmt.Errorf("幂等重放失败: %w", err)
	}
	if !replay.Replay || replay.Batch.ID != batchID {
		return fmt.Errorf("创建批次未返回稳定幂等结果")
	}
	permit := map[string]any{"expectedVersion": 1, "idempotencyKey": "selfcheck-permit", "permitNumber": "PERMIT-SC", "validFrom": now.Add(-24 * time.Hour), "validUntil": now.Add(24 * time.Hour), "allowedMaterialCodes": []string{"PLANT"}, "quantityLimit": 10, "issuer": "保护地管理机构"}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/permits", "selfcheck-collector", application.RoleCollector, permit, http.StatusOK, &result); err != nil {
		return err
	}
	specimen := map[string]any{"expectedVersion": 2, "idempotencyKey": "selfcheck-specimen", "materialCode": "PLANT", "sourceDescription": "样方 A 植物组织", "collectedAt": now, "containerCode": "CONT-SC-01", "sealCode": "SEAL-SC-01", "preservationRequirement": "cold", "quantity": 2}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/specimens", "selfcheck-collector", application.RoleCollector, specimen, http.StatusOK, &result); err != nil {
		return err
	}
	specimenID := result.Batch.Specimens[0].ID
	var readiness struct {
		Ready bool `json:"ready"`
	}
	if err := c.call(ctx, http.MethodGet, "/api/v1/batches/"+batchID+"/departure-readiness", "selfcheck-collector", application.RoleCollector, nil, http.StatusOK, &readiness); err != nil || !readiness.Ready {
		return fmt.Errorf("离场就绪预检失败: %w", err)
	}
	departure := map[string]any{"expectedVersion": 3, "idempotencyKey": "selfcheck-departure"}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/departure-verification", "selfcheck-collector", application.RoleCollector, departure, http.StatusOK, &result); err != nil {
		return err
	}
	handoff := map[string]any{"expectedVersion": 4, "idempotencyKey": "selfcheck-handoff", "sequence": 1, "releasedBy": "野外采集负责人", "receivedBy": "冷链运输员", "occurredAt": now.Add(time.Minute), "location": "保护地出口", "sealCondition": "intact", "temperatureSummary": "2-6C"}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/handoffs", "selfcheck-custodian", application.RoleCustodian, handoff, http.StatusOK, &result); err != nil {
		return err
	}
	received := []map[string]any{{"specimenId": specimenID, "containerCode": "CONT-SC-01", "sealCode": "SEAL-SC-01", "sealCondition": "intact", "preservationCondition": "cold", "quantity": 1}}
	arrival := map[string]any{"expectedVersion": 5, "idempotencyKey": "selfcheck-arrival", "received": received}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/arrival-inspections", "selfcheck-receiver", application.RoleReceiver, arrival, http.StatusOK, &result); err != nil {
		return err
	}
	if len(result.Batch.Discrepancies) != 1 {
		return fmt.Errorf("到站差异分支未生成预期问题")
	}
	issueID := result.Batch.Discrepancies[0].ID
	remediation := map[string]any{"expectedVersion": 6, "idempotencyKey": "selfcheck-remediation", "remediationNote": "补齐缺少的一份并重新核验容器", "evidenceDigest": "sha256:selfcheck-evidence"}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/discrepancies/"+issueID+"/remediation", "selfcheck-collector", application.RoleCollector, remediation, http.StatusOK, &result); err != nil {
		return err
	}
	review := map[string]any{"expectedVersion": 7, "idempotencyKey": "selfcheck-review", "revision": 1, "approved": true, "opinion": "证据与实物复核一致"}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/discrepancies/"+issueID+"/review", "selfcheck-reviewer", application.RoleReceiver, review, http.StatusOK, &result); err != nil {
		return err
	}
	reverify := map[string]any{"expectedVersion": 8, "idempotencyKey": "selfcheck-reverify"}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/arrival-reverification", "selfcheck-reviewer", application.RoleReceiver, reverify, http.StatusOK, &result); err != nil {
		return err
	}
	approve := map[string]any{"expectedVersion": 9, "idempotencyKey": "selfcheck-approve", "approvedBy": "合规复核员"}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/compliance-approval", "selfcheck-compliance", application.RoleCompliance, approve, http.StatusOK, &result); err != nil {
		return err
	}
	var verification application.CertificateVerification
	if err := c.call(ctx, http.MethodGet, "/api/v1/batches/"+batchID+"/certificate", "selfcheck-compliance", "", nil, http.StatusOK, &verification); err != nil {
		return err
	}
	if !verification.OverallValid || verification.Certificate == nil || !verification.VerificationDigest.Valid || !verification.ManifestDigest.Valid || !verification.QuantityAndVersion.Valid || !verification.IssuanceEvent.Valid {
		return fmt.Errorf("入库凭证校验失败")
	}
	frozenChange := map[string]any{"expectedVersion": 10, "idempotencyKey": "selfcheck-frozen", "materialCode": "PLANT", "sourceDescription": "冻结后修改", "collectedAt": now, "containerCode": "CONT-X", "sealCode": "SEAL-X", "preservationRequirement": "cold", "quantity": 1}
	if err := c.call(ctx, http.MethodPost, "/api/v1/batches/"+batchID+"/specimens", "selfcheck-collector", application.RoleCollector, frozenChange, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("冻结校验失败: %w", err)
	}
	var timeline struct {
		Timeline []json.RawMessage `json:"timeline"`
	}
	if err := c.call(ctx, http.MethodGet, "/api/v1/batches/"+batchID+"/timeline", "selfcheck-compliance", "", nil, http.StatusOK, &timeline); err != nil {
		return err
	}
	if len(timeline.Timeline) != 10 {
		return fmt.Errorf("时间线事件数量异常: %d", len(timeline.Timeline))
	}
	return nil
}

func (c *selfcheckClient) call(ctx context.Context, method, path, actor, role string, payload any, expected int, output any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	c.serial++
	request.Header.Set("X-Actor", actor)
	request.Header.Set("X-Role", role)
	request.Header.Set("X-Request-ID", fmt.Sprintf("selfcheck-%d", c.serial))
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != expected {
		return fmt.Errorf("%s %s 期望状态 %d，实际 %d: %s", method, path, expected, response.StatusCode, string(data))
	}
	if output != nil && len(data) > 0 {
		if err := json.Unmarshal(data, output); err != nil {
			return fmt.Errorf("解析 %s %s 响应: %w", method, path, err)
		}
	}
	return nil
}
