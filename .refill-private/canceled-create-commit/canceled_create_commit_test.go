package canceledcreatecommit_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

type cancelBoundaryStore struct {
	*persistence.Store
	requestContext context.Context
	commitEntered  chan struct{}
}

func (s *cancelBoundaryStore) Commit(request persistence.CommitRequest) (json.RawMessage, bool, error) {
	close(s.commitEntered)
	<-s.requestContext.Done()
	return s.Store.Commit(request)
}

func (s *cancelBoundaryStore) CommitContext(ctx context.Context, _ persistence.CommitRequest) (json.RawMessage, bool, error) {
	close(s.commitEntered)
	<-ctx.Done()
	return nil, false, ctx.Err()
}

func TestCanceledCreateDoesNotCommit(t *testing.T) {
	realStore, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	store := &cancelBoundaryStore{
		Store:          realStore,
		requestContext: ctx,
		commitEntered:  make(chan struct{}),
	}
	handler := httpapi.New(
		application.NewService(store),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	).Handler()
	body := `{"expectedVersion":0,"idempotencyKey":"cancel-create-key","batchCode":"CANCEL-001","collectionSite":"保护区东坡","destinationRepository":"样本库","leadCollector":"采集员"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/batches", strings.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor", "collector-a")
	request.Header.Set("X-Role", application.RoleCollector)
	request.Header.Set("X-Request-ID", "cancel-create-request")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(response, request)
	}()

	<-store.commitEntered
	cancel()
	<-done

	batchID := responseBatchID("cancel-create-key")
	_, lookupErr := realStore.Get(batchID)
	if response.Code >= 200 && response.Code < 300 || lookupErr == nil {
		t.Fatalf("已取消的创建请求仍返回 %d 且提交状态（lookupErr=%v）", response.Code, lookupErr)
	}
}

func responseBatchID(idempotencyKey string) string {
	hash := sha256.Sum256([]byte(idempotencyKey))
	return "batch-" + hex.EncodeToString(hash[:12])
}
