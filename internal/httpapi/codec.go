package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/domain"
)

const maxRequestBody = 1 << 20

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string                   `json:"code"`
	Message string                   `json:"message"`
	Issues  []domain.ValidationIssue `json:"issues,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return &domain.FieldError{Issues: []domain.ValidationIssue{domain.NewIssue("content_type", "Content-Type", "必须使用 application/json")}}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &domain.FieldError{Issues: []domain.ValidationIssue{domain.NewIssue("invalid_json", "body", "JSON 请求体无效: "+err.Error())}}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &domain.FieldError{Issues: []domain.ValidationIssue{domain.NewIssue("multiple_values", "body", "请求体只能包含一个 JSON 对象")}}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	message := "服务处理请求时发生错误"
	var field *domain.FieldError
	detail := errorDetail{}
	switch {
	case errors.As(err, &field):
		status, code, message, detail.Issues = http.StatusUnprocessableEntity, "validation_failed", err.Error(), field.Issues
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", err.Error()
	case errors.Is(err, domain.ErrVersionConflict):
		status, code, message = http.StatusConflict, "version_conflict", err.Error()
	case errors.Is(err, domain.ErrIdempotencyConflict):
		status, code, message = http.StatusConflict, "idempotency_conflict", err.Error()
	case errors.Is(err, domain.ErrInvalidTransition):
		status, code, message = http.StatusConflict, "invalid_transition", err.Error()
	case errors.Is(err, domain.ErrFrozen):
		status, code, message = http.StatusConflict, "manifest_frozen", err.Error()
	case errors.Is(err, domain.ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", err.Error()
	}
	detail.Code, detail.Message = code, message
	writeJSON(w, status, errorResponse{Error: detail})
}

func populateMeta(r *http.Request, meta *application.CommandMeta) {
	meta.Actor = strings.TrimSpace(r.Header.Get("X-Actor"))
	meta.Role = strings.TrimSpace(r.Header.Get("X-Role"))
	meta.RequestID = strings.TrimSpace(r.Header.Get("X-Request-ID"))
}
