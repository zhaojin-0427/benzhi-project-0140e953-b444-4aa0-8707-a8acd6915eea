package httpapi

import (
	"net/http"

	"specimen-custody-gate/internal/application"
)

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var command application.CreateBatchCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &command.CommandMeta)
	result, err := s.service.CreateBatchContext(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) ListBatches(w http.ResponseWriter, _ *http.Request) {
	result, err := s.service.ListBatches()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": result})
}

func (s *Server) GetBatch(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.GetBatch(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batch": result})
}

func (s *Server) RegisterPermit(w http.ResponseWriter, r *http.Request) {
	var command application.RegisterPermitCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &command.CommandMeta)
	result, err := s.service.RegisterPermit(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) RegisterSpecimen(w http.ResponseWriter, r *http.Request) {
	var command application.RegisterSpecimenCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &command.CommandMeta)
	result, err := s.service.RegisterSpecimen(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) VerifyDeparture(w http.ResponseWriter, r *http.Request) {
	var meta application.CommandMeta
	if err := decodeJSON(w, r, &meta); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &meta)
	result, err := s.service.VerifyDeparture(r.PathValue("batchID"), meta)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) DepartureReadiness(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.DepartureReadiness(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) RecordHandoff(w http.ResponseWriter, r *http.Request) {
	var command application.HandoffCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &command.CommandMeta)
	result, err := s.service.RecordHandoff(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) InspectArrival(w http.ResponseWriter, r *http.Request) {
	var command application.ArrivalCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &command.CommandMeta)
	result, err := s.service.InspectArrival(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
