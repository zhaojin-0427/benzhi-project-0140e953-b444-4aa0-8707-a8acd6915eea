package domain

import "time"

type BatchStatus string

const (
	StatusDraft               BatchStatus = "draft"
	StatusCustodyInTransit    BatchStatus = "custody_in_transit"
	StatusRemediationRequired BatchStatus = "remediation_required"
	StatusReviewPending       BatchStatus = "review_pending"
	StatusDeposited           BatchStatus = "deposited"
)

type TransferBatch struct {
	ID                    string              `json:"id"`
	BatchCode             string              `json:"batchCode"`
	Status                BatchStatus         `json:"status"`
	CollectionSite        string              `json:"collectionSite"`
	DestinationRepository string              `json:"destinationRepository"`
	LeadCollector         string              `json:"leadCollector"`
	Version               int                 `json:"version"`
	CreatedAt             time.Time           `json:"createdAt"`
	UpdatedAt             time.Time           `json:"updatedAt"`
	Permits               []CollectionPermit  `json:"permits"`
	Specimens             []Specimen          `json:"specimens"`
	Handoffs              []CustodyHandoff    `json:"handoffs"`
	Discrepancies         []Discrepancy       `json:"discrepancies"`
	Certificate           *DepositCertificate `json:"certificate,omitempty"`
	ManifestDigest        string              `json:"manifestDigest,omitempty"`
	PermitQuota           []PermitQuotaUsage  `json:"permitQuota"`
	PermitWarnings        []PermitWarning     `json:"permitWarnings"`
	TransitOverview       TransitOverview     `json:"transitOverview"`
}

type CollectionPermit struct {
	ID                   string    `json:"id"`
	BatchID              string    `json:"batchId"`
	PermitNumber         string    `json:"permitNumber"`
	ValidFrom            time.Time `json:"validFrom"`
	ValidUntil           time.Time `json:"validUntil"`
	AllowedMaterialCodes []string  `json:"allowedMaterialCodes"`
	QuantityLimit        int       `json:"quantityLimit"`
	Issuer               string    `json:"issuer"`
}

type Specimen struct {
	ID                      string    `json:"id"`
	BatchID                 string    `json:"batchId"`
	MaterialCode            string    `json:"materialCode"`
	SourceDescription       string    `json:"sourceDescription"`
	CollectedAt             time.Time `json:"collectedAt"`
	ContainerCode           string    `json:"containerCode"`
	SealCode                string    `json:"sealCode"`
	PreservationRequirement string    `json:"preservationRequirement"`
	Quantity                int       `json:"quantity"`
}

type HandoffStatus string

const (
	HandoffOpen      HandoffStatus = "open"
	HandoffConfirmed HandoffStatus = "confirmed"
)

type CustodyHandoff struct {
	ID                 string        `json:"id"`
	BatchID            string        `json:"batchId"`
	Sequence           int           `json:"sequence"`
	ReleasedBy         string        `json:"releasedBy"`
	ReceivedBy         string        `json:"receivedBy"`
	OccurredAt         time.Time     `json:"occurredAt"`
	Location           string        `json:"location"`
	SealCondition      string        `json:"sealCondition"`
	TemperatureSummary string        `json:"temperatureSummary"`
	Status             HandoffStatus `json:"status"`
}

type DiscrepancyStatus string

const (
	DiscrepancyOpen       DiscrepancyStatus = "open"
	DiscrepancyRemediated DiscrepancyStatus = "remediated"
	DiscrepancyClosed     DiscrepancyStatus = "closed"
	DiscrepancyRejected   DiscrepancyStatus = "rejected"
)

type Discrepancy struct {
	ID              string                `json:"id"`
	BatchID         string                `json:"batchId"`
	SpecimenID      string                `json:"specimenId,omitempty"`
	Category        string                `json:"category"`
	Description     string                `json:"description"`
	Status          DiscrepancyStatus     `json:"status"`
	RemediationNote string                `json:"remediationNote,omitempty"`
	EvidenceDigest  string                `json:"evidenceDigest,omitempty"`
	ReviewedBy      string                `json:"reviewedBy,omitempty"`
	ReviewedAt      *time.Time            `json:"reviewedAt,omitempty"`
	Revisions       []RemediationRevision `json:"revisions"`
	LatestActions   []string              `json:"latestActions"`
}

type RemediationRevision struct {
	Revision       int                `json:"revision"`
	SubmittedBy    string             `json:"submittedBy"`
	Note           string             `json:"note"`
	EvidenceDigest string             `json:"evidenceDigest"`
	SubmittedAt    time.Time          `json:"submittedAt"`
	Review         *RemediationReview `json:"review,omitempty"`
}

type RemediationReview struct {
	ReviewedBy string    `json:"reviewedBy"`
	Approved   bool      `json:"approved"`
	Opinion    string    `json:"opinion"`
	ReviewedAt time.Time `json:"reviewedAt"`
}

type DepositCertificate struct {
	ID                 string    `json:"id"`
	BatchID            string    `json:"batchId"`
	BatchVersion       int       `json:"batchVersion"`
	ManifestDigest     string    `json:"manifestDigest"`
	SpecimenCount      int       `json:"specimenCount"`
	ApprovedBy         string    `json:"approvedBy"`
	IssuedAt           time.Time `json:"issuedAt"`
	VerificationDigest string    `json:"verificationDigest"`
}

type ValidationIssue struct {
	Code              string `json:"code"`
	Field             string `json:"field,omitempty"`
	SpecimenID        string `json:"specimenId,omitempty"`
	Expected          string `json:"expected,omitempty"`
	Actual            string `json:"actual,omitempty"`
	ShortfallQuantity int    `json:"shortfallQuantity,omitempty"`
	Message           string `json:"message"`
}

type PermitQuotaUsage struct {
	PermitID      string `json:"permitId"`
	PermitNumber  string `json:"permitNumber"`
	MaterialCode  string `json:"materialCode"`
	QuantityLimit int    `json:"quantityLimit"`
	UsedQuantity  int    `json:"usedQuantity"`
	Remaining     int    `json:"remaining"`
}

type PermitWarning struct {
	Code              string `json:"code"`
	MaterialCode      string `json:"materialCode"`
	SpecimenID        string `json:"specimenId"`
	ShortfallQuantity int    `json:"shortfallQuantity"`
	Message           string `json:"message"`
}

type TransitOverview struct {
	CurrentCustodian        string     `json:"currentCustodian,omitempty"`
	LastHandoffLocation     string     `json:"lastHandoffLocation,omitempty"`
	LastHandoffAt           *time.Time `json:"lastHandoffAt,omitempty"`
	SealAnomalyCount        int        `json:"sealAnomalyCount"`
	TemperatureAnomalyCount int        `json:"temperatureAnomalyCount"`
}
