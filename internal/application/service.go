package application

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"specimen-custody-gate/internal/domain"
	"specimen-custody-gate/internal/persistence"
)

type serviceStore interface {
	Get(string) (*domain.TransferBatch, error)
	List() ([]domain.TransferBatch, error)
	LookupIdempotency(string, string) (json.RawMessage, bool, error)
	Commit(persistence.CommitRequest) (json.RawMessage, bool, error)
	RecordsForBatch(string) []persistence.LogRecord
	InspectCertificateIssuance(string, domain.DepositCertificate) persistence.CertificateIssuanceInspection
}

type Service struct {
	mu    sync.Mutex
	store serviceStore
	now   func() time.Time
	id    func(string) string
}

func NewService(store serviceStore) *Service {
	return &Service{store: store, now: time.Now, id: newID}
}

func newID(prefix string) string {
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return prefix + "-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + "-" + hex.EncodeToString(random)
}

func validateMeta(meta CommandMeta) error {
	issues := []domain.ValidationIssue{}
	domain.RequireText(meta.Actor, "X-Actor", &issues)
	domain.RequireText(meta.Role, "X-Role", &issues)
	domain.RequireText(meta.RequestID, "X-Request-ID", &issues)
	domain.RequireText(meta.IdempotencyKey, "idempotencyKey", &issues)
	if meta.ExpectedVersion < 0 {
		issues = append(issues, domain.NewIssue("invalid", "expectedVersion", "版本不能为负数"))
	}
	if len(issues) > 0 {
		return &domain.FieldError{Issues: issues}
	}
	return nil
}

func requireRole(actual string, allowed ...string) error {
	for _, role := range allowed {
		if actual == role {
			return nil
		}
	}
	return domain.ErrForbidden
}

func (s *Service) GetBatch(id string) (*domain.TransferBatch, error) { return s.store.Get(id) }

func (s *Service) ListBatches() ([]domain.TransferBatch, error) { return s.store.List() }

func IsKnownError(err error) bool {
	if err == nil {
		return false
	}
	var field *domain.FieldError
	return errors.As(err, &field) || errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrVersionConflict) || errors.Is(err, domain.ErrIdempotencyConflict) || errors.Is(err, domain.ErrInvalidTransition) || errors.Is(err, domain.ErrFrozen) || errors.Is(err, domain.ErrForbidden)
}
