package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"specimen-custody-gate/internal/application"
)

type Server struct {
	service *application.Service
	logger  *slog.Logger
	mux     *http.ServeMux
}

func New(service *application.Service, logger *slog.Logger) *Server {
	server := &Server{service: service, logger: logger, mux: http.NewServeMux()}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return requestMiddleware(s.logger, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.Health)
	s.mux.HandleFunc("POST /api/v1/batches", s.CreateBatch)
	s.mux.HandleFunc("GET /api/v1/batches", s.ListBatches)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}", s.GetBatch)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/permits", s.RegisterPermit)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/specimens", s.RegisterSpecimen)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/departure-verification", s.VerifyDeparture)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}/departure-readiness", s.DepartureReadiness)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/handoffs", s.RecordHandoff)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/arrival-inspections", s.InspectArrival)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}/discrepancies", s.ListDiscrepancies)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/discrepancies/{discrepancyID}/remediation", s.SubmitRemediation)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/discrepancies/{discrepancyID}/review", s.ReviewDiscrepancy)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/arrival-reverification", s.ReverifyArrival)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/compliance-approval", s.ApproveDeposit)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}/timeline", s.GetTimeline)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}/certificate", s.GetCertificate)
}

func requestMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
		logger.Info("HTTP 请求完成", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started), "requestId", r.Header.Get("X-Request-ID"))
	})
}
