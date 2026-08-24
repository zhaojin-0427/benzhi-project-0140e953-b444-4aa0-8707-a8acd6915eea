package persistence

import (
	"encoding/json"

	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
)

const SchemaVersion = 1

type LogRecord struct {
	SchemaVersion  int             `json:"schemaVersion"`
	Sequence       uint64          `json:"sequence"`
	PreviousDigest string          `json:"previousDigest"`
	Event          domain.Event    `json:"event"`
	Audit          audit.Record    `json:"audit"`
	IdempotencyKey string          `json:"idempotencyKey"`
	RequestDigest  string          `json:"requestDigest"`
	Response       json.RawMessage `json:"response"`
	Digest         string          `json:"digest"`
}

type digestMaterial struct {
	SchemaVersion  int             `json:"schemaVersion"`
	Sequence       uint64          `json:"sequence"`
	PreviousDigest string          `json:"previousDigest"`
	Event          domain.Event    `json:"event"`
	Audit          audit.Record    `json:"audit"`
	IdempotencyKey string          `json:"idempotencyKey"`
	RequestDigest  string          `json:"requestDigest"`
	Response       json.RawMessage `json:"response"`
}

func (r LogRecord) canonical() ([]byte, error) {
	return json.Marshal(digestMaterial{SchemaVersion: r.SchemaVersion, Sequence: r.Sequence, PreviousDigest: r.PreviousDigest, Event: r.Event, Audit: r.Audit, IdempotencyKey: r.IdempotencyKey, RequestDigest: r.RequestDigest, Response: r.Response})
}
