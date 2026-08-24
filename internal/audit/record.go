package audit

import (
	"strings"
	"time"
)

type Record struct {
	Actor          string    `json:"actor"`
	Role           string    `json:"role"`
	RequestID      string    `json:"requestId"`
	IdempotencyKey string    `json:"idempotencyKey"`
	BatchID        string    `json:"batchId"`
	BatchVersion   int       `json:"batchVersion"`
	Action         string    `json:"action"`
	Result         string    `json:"result"`
	OccurredAt     time.Time `json:"occurredAt"`
}

type Context struct {
	Actor          string
	Role           string
	RequestID      string
	IdempotencyKey string
}

func Build(ctx Context, batchID, action, result string, version int, at time.Time) Record {
	return Record{
		Actor: strings.TrimSpace(ctx.Actor), Role: strings.TrimSpace(ctx.Role),
		RequestID: strings.TrimSpace(ctx.RequestID), IdempotencyKey: strings.TrimSpace(ctx.IdempotencyKey),
		BatchID: batchID, BatchVersion: version, Action: strings.TrimSpace(action),
		Result: strings.TrimSpace(result), OccurredAt: at.UTC(),
	}
}

func (r Record) Complete() bool {
	return r.Actor != "" && r.Role != "" && r.RequestID != "" && r.IdempotencyKey != "" && r.BatchID != "" && r.Action != "" && r.Result != ""
}
