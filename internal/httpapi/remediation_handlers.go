package httpapi

import (
	"net/http"
	"strings"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/domain"
)

func (s *Server) ListDiscrepancies(w http.ResponseWriter, r *http.Request) {
	filter, err := discrepancyFilter(r)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.ListDiscrepancies(r.PathValue("batchID"), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func discrepancyFilter(r *http.Request) (domain.DiscrepancyFilter, error) {
	query := r.URL.Query()
	issues := []domain.ValidationIssue{}
	readSingle := func(name string) (string, bool) {
		values, present := query[name]
		if !present {
			return "", false
		}
		if len(values) != 1 {
			issues = append(issues, domain.NewIssue("multiple_values", name, "查询参数只能提供一个值"))
			return "", true
		}
		return strings.TrimSpace(values[0]), true
	}
	status, hasStatus := readSingle("status")
	category, hasCategory := readSingle("category")
	specimenID, hasSpecimenID := readSingle("specimenId")
	if hasStatus && !domain.KnownDiscrepancyStatus(status) {
		issues = append(issues, domain.NewIssue("unknown", "status", "未知的问题状态"))
	}
	if hasCategory && !domain.KnownDiscrepancyCategory(category) {
		issues = append(issues, domain.NewIssue("unknown", "category", "未知的问题类别"))
	}
	if hasSpecimenID && specimenID == "" {
		issues = append(issues, domain.NewIssue("required", "specimenId", "样本编号查询参数不能为空"))
	}
	if len(issues) > 0 {
		return domain.DiscrepancyFilter{}, &domain.FieldError{Issues: issues}
	}
	return domain.DiscrepancyFilter{Status: status, Category: category, SpecimenID: specimenID}, nil
}

func (s *Server) SubmitRemediation(w http.ResponseWriter, r *http.Request) {
	var command application.RemediationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &command.CommandMeta)
	result, err := s.service.SubmitRemediation(r.PathValue("batchID"), r.PathValue("discrepancyID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) ReviewDiscrepancy(w http.ResponseWriter, r *http.Request) {
	var command application.ReviewCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &command.CommandMeta)
	result, err := s.service.ReviewDiscrepancy(r.PathValue("batchID"), r.PathValue("discrepancyID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) ReverifyArrival(w http.ResponseWriter, r *http.Request) {
	var meta application.CommandMeta
	if err := decodeJSON(w, r, &meta); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &meta)
	result, err := s.service.ReverifyArrival(r.PathValue("batchID"), meta)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) ApproveDeposit(w http.ResponseWriter, r *http.Request) {
	var command application.ApproveCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	populateMeta(r, &command.CommandMeta)
	result, err := s.service.ApproveDeposit(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) GetTimeline(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.Timeline(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timeline": result})
}

func (s *Server) GetCertificate(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.VerifyCertificate(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
