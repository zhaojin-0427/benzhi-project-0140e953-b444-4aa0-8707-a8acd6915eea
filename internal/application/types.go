package application

import (
	"time"

	"specimen-custody-gate/internal/domain"
)

const (
	RoleCollector  = "collector"
	RoleCustodian  = "custodian"
	RoleReceiver   = "receiver"
	RoleCompliance = "compliance"
)

type CommandMeta struct {
	ExpectedVersion int    `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"-"`
	Role            string `json:"-"`
	RequestID       string `json:"-"`
}

type CreateBatchCommand struct {
	CommandMeta
	BatchCode             string `json:"batchCode"`
	CollectionSite        string `json:"collectionSite"`
	DestinationRepository string `json:"destinationRepository"`
	LeadCollector         string `json:"leadCollector"`
}

type RegisterPermitCommand struct {
	CommandMeta
	PermitNumber         string    `json:"permitNumber"`
	ValidFrom            time.Time `json:"validFrom"`
	ValidUntil           time.Time `json:"validUntil"`
	AllowedMaterialCodes []string  `json:"allowedMaterialCodes"`
	QuantityLimit        int       `json:"quantityLimit"`
	Issuer               string    `json:"issuer"`
}

type RegisterSpecimenCommand struct {
	CommandMeta
	MaterialCode            string    `json:"materialCode"`
	SourceDescription       string    `json:"sourceDescription"`
	CollectedAt             time.Time `json:"collectedAt"`
	ContainerCode           string    `json:"containerCode"`
	SealCode                string    `json:"sealCode"`
	PreservationRequirement string    `json:"preservationRequirement"`
	Quantity                int       `json:"quantity"`
}

type HandoffCommand struct {
	CommandMeta
	Sequence           int       `json:"sequence"`
	ReleasedBy         string    `json:"releasedBy"`
	ReceivedBy         string    `json:"receivedBy"`
	OccurredAt         time.Time `json:"occurredAt"`
	Location           string    `json:"location"`
	SealCondition      string    `json:"sealCondition"`
	TemperatureSummary string    `json:"temperatureSummary"`
}

type ArrivalCommand struct {
	CommandMeta
	Received []domain.ReceivedSpecimen `json:"received"`
}

type RemediationCommand struct {
	CommandMeta
	RemediationNote string `json:"remediationNote"`
	EvidenceDigest  string `json:"evidenceDigest"`
}

type ReviewCommand struct {
	CommandMeta
	Revision int    `json:"revision"`
	Approved bool   `json:"approved"`
	Opinion  string `json:"opinion"`
}

type ApproveCommand struct {
	CommandMeta
	ApprovedBy string `json:"approvedBy"`
}

type BatchResult struct {
	Batch     *domain.TransferBatch `json:"batch"`
	Replay    bool                  `json:"idempotentReplay"`
	EventType string                `json:"eventType"`
}

type VerificationCheck struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
}

type CertificateVerification struct {
	Certificate        *domain.DepositCertificate `json:"certificate"`
	OverallValid       bool                       `json:"overallValid"`
	Valid              bool                       `json:"valid"`
	VerificationDigest VerificationCheck          `json:"verificationDigest"`
	ManifestDigest     VerificationCheck          `json:"manifestDigest"`
	QuantityAndVersion VerificationCheck          `json:"quantityAndVersion"`
	IssuanceEvent      VerificationCheck          `json:"issuanceEvent"`
}
