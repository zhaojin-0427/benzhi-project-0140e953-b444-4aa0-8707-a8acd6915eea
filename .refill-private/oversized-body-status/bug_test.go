package oversized_body_status_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/httpapi"
	"specimen-custody-gate/internal/persistence"
)

func TestOversizedJSONReturnsPayloadTooLarge(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(
		application.NewService(store),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	).Handler()
	body := `{"expectedVersion":0,"idempotencyKey":"large","batchCode":"` + strings.Repeat("x", (1<<20)+1) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/batches", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor", "采集员")
	request.Header.Set("X-Role", application.RoleCollector)
	request.Header.Set("X-Request-ID", "oversized-request")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("TestOversizedJSONReturnsPayloadTooLarge: 超过公开限制的请求应返回 413，实际 %d: %s", response.Code, response.Body.String())
	}
}
