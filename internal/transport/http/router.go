package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gradeflow/reporting-service/internal/domain"
	"gradeflow/reporting-service/internal/usecase"
)

type Handler struct {
	reports *usecase.ReportService
	logger  *slog.Logger
}

func NewRouter(reports *usecase.ReportService, logger *slog.Logger) http.Handler {
	h := &Handler{reports: reports, logger: logger}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(h.logRequest)

	r.Get("/healthz", h.health)
	r.Route("/api/v1/reports", func(r chi.Router) {
		r.Get("/{code}/template", h.getTemplate)
		r.Post("/{code}/generate", h.generate)
		r.Post("/{code}/export/{format}", h.export)
	})
	return r
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) getTemplate(w http.ResponseWriter, r *http.Request) {
	template, err := h.reports.GetTemplate(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeReportRequest(w, r)
	if !ok {
		return
	}
	result, err := h.reports.Generate(r.Context(), chi.URLParam(r, "code"), req)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeReportRequest(w, r)
	if !ok {
		return
	}
	filename, contentType, reader, err := h.reports.Export(r.Context(), chi.URLParam(r, "code"), strings.ToLower(chi.URLParam(r, "format")), req)
	if err != nil {
		h.writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

func decodeReportRequest(w http.ResponseWriter, r *http.Request) (domain.ReportRequest, bool) {
	defer r.Body.Close()
	var req domain.ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json", "message": err.Error()})
		return domain.ReportRequest{}, false
	}
	return req, true
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
	case errors.Is(err, domain.ErrInvalidRequest), errors.Is(err, domain.ErrInvalidTemplate):
		status = http.StatusBadRequest
		code = "bad_request"
	case errors.Is(err, domain.ErrUnsupportedExport):
		status = http.StatusUnsupportedMediaType
		code = "unsupported_export"
	}
	if status >= 500 {
		h.logger.Error("request failed", "error", err)
	}
	writeJSON(w, status, map[string]string{"error": code, "message": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (h *Handler) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		h.logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
	})
}
