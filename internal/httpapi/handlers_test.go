package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/audit"
	"specimen-custody-gate/internal/domain"
	"specimen-custody-gate/internal/persistence"
)

func TestCreateRejectsUnknownField(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := New(application.NewService(store), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	body := `{"expectedVersion":0,"idempotencyKey":"key","batchCode":"B","collectionSite":"地点","destinationRepository":"库","leadCollector":"人","unknown":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/batches", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor", "actor")
	request.Header.Set("X-Role", application.RoleCollector)
	request.Header.Set("X-Request-ID", "request")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("期望 422，实际 %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "invalid_json") {
		t.Fatalf("错误响应缺少机器代码: %s", response.Body.String())
	}
}

func TestCreateRequiresJSONContentType(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := New(application.NewService(store), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/batches", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("期望 422，实际 %d", response.Code)
	}
}

func TestDiscrepancyQueryRejectsUnknownStatusWithoutMutation(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	event, err := domain.CreateBatch(domain.CreateBatchInput{ID: "b", BatchCode: "B", CollectionSite: "地点", DestinationRepository: "库", LeadCollector: "人"}, now)
	if err != nil {
		t.Fatal(err)
	}
	record := audit.Build(audit.Context{Actor: "actor", Role: "collector", RequestID: "request", IdempotencyKey: "key"}, "b", "create_batch", "accepted", 1, now)
	if _, _, err := store.Commit(persistence.CommitRequest{ExpectedVersion: 0, Event: event, Audit: record, IdempotencyKey: "key", RequestDigest: "digest", Response: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	handler := New(application.NewService(store), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/batches/b/discrepancies?status=unknown", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"field":"status"`) {
		t.Fatalf("无效状态响应异常: %d %s", response.Code, response.Body.String())
	}
	batch, err := store.Get("b")
	if err != nil || batch.Version != 1 {
		t.Fatalf("只读查询改变批次: %+v %v", batch, err)
	}
}
