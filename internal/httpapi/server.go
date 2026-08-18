package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/middleware"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/requestmeta"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/service"
)

type Server struct {
	services *service.Services
	logger   *slog.Logger
	store    repository.Store
}

func New(services *service.Services, store repository.Store, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{services: services, store: store, logger: logger}
	return middleware.Logging(logger, middleware.RequestContext(middleware.Recovery(logger, s.routes())))
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.Handle("POST /api/v1/auth/logout", s.auth(http.HandlerFunc(s.logout)))
	mux.Handle("POST /api/v1/users", s.auth(http.HandlerFunc(s.createUser)))
	mux.Handle("POST /api/v1/studies", s.auth(http.HandlerFunc(s.createStudy)))
	mux.Handle("POST /api/v1/studies/{id}/activate", s.auth(http.HandlerFunc(s.activateStudy)))
	mux.Handle("POST /api/v1/sites", s.auth(http.HandlerFunc(s.createSite)))
	mux.Handle("POST /api/v1/containers", s.auth(http.HandlerFunc(s.createContainer)))
	mux.Handle("POST /api/v1/containers/{id}/cleaning/start", s.auth(http.HandlerFunc(s.startContainerCleaning)))
	mux.Handle("POST /api/v1/containers/{id}/cleaning/complete", s.auth(http.HandlerFunc(s.completeContainerCleaning)))
	mux.Handle("POST /api/v1/containers/{id}/retire", s.auth(http.HandlerFunc(s.retireContainer)))
	mux.Handle("POST /api/v1/samples", s.auth(http.HandlerFunc(s.registerSample)))
	mux.Handle("POST /api/v1/samples/bulk", s.auth(http.HandlerFunc(s.bulkRegisterSamples)))
	mux.Handle("POST /api/v1/samples/{id}/ready", s.auth(http.HandlerFunc(s.readySample)))
	mux.Handle("POST /api/v1/shipments", s.auth(http.HandlerFunc(s.planShipment)))
	mux.Handle("GET /api/v1/shipments", s.auth(http.HandlerFunc(s.listShipments)))
	mux.Handle("GET /api/v1/shipments/{id}", s.auth(http.HandlerFunc(s.getShipment)))
	mux.Handle("GET /api/v1/shipments/{id}/reconciliation", s.auth(http.HandlerFunc(s.reconcileShipment)))
	mux.Handle("POST /api/v1/shipments/{id}/pack", s.auth(http.HandlerFunc(s.packShipment)))
	mux.Handle("POST /api/v1/shipments/{id}/dispatch", s.auth(http.HandlerFunc(s.dispatchShipment)))
	mux.Handle("POST /api/v1/shipments/{id}/arrive", s.auth(http.HandlerFunc(s.arriveShipment)))
	mux.Handle("POST /api/v1/shipments/{id}/close", s.auth(http.HandlerFunc(s.closeShipment)))
	mux.Handle("POST /api/v1/shipments/{id}/cancel", s.auth(http.HandlerFunc(s.cancelShipment)))
	mux.Handle("POST /api/v1/shipments/{id}/handoffs", s.auth(http.HandlerFunc(s.createHandoff)))
	mux.Handle("POST /api/v1/handoffs/{id}/resolve", s.auth(http.HandlerFunc(s.resolveHandoff)))
	mux.Handle("POST /api/v1/shipments/{id}/readings", s.auth(http.HandlerFunc(s.recordReading)))
	mux.Handle("GET /api/v1/excursions", s.auth(http.HandlerFunc(s.listExcursions)))
	mux.Handle("POST /api/v1/excursions/{id}/review", s.auth(http.HandlerFunc(s.startReview)))
	mux.Handle("POST /api/v1/excursions/{id}/decision", s.auth(http.HandlerFunc(s.decideExcursion)))
	mux.Handle("GET /api/v1/audit", s.auth(http.HandlerFunc(s.listAudit)))
	mux.Handle("GET /api/v1/summary", s.auth(http.HandlerFunc(s.summary)))
	return mux
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, r, fmt.Errorf("bearer token required: %w", domain.ErrValidation))
			return
		}
		principal, err := s.services.Auth.Authenticate(r.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(requestmeta.WithPrincipal(r.Context(), principal)))
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, r, fmt.Errorf("database unavailable: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func decodeJSON(r *http.Request, target any) error {
	body, readErr := io.ReadAll(io.LimitReader(r.Body, 2<<20+1))
	if readErr != nil {
		return domain.FieldError{Field: "body", Message: "could not be read"}
	}
	if len(body) > 2<<20 {
		return domain.FieldError{Field: "body", Message: "is too large"}
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.FieldError{Field: "body", Message: "must be valid JSON"}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return domain.FieldError{Field: "body", Message: "must contain one JSON value"}
	}
	return nil
}

func parseID(r *http.Request) string { return strings.TrimSpace(r.PathValue("id")) }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := classifyError(err)
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": requestmeta.RequestID(r.Context())}})
}

func classifyError(err error) (int, string, string) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		return http.StatusBadRequest, "invalid_request", err.Error()
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "not_found", "resource not found"
	case errors.Is(err, domain.ErrVersionConflict):
		return http.StatusConflict, "version_conflict", "resource changed; reload and retry"
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrAlreadyExists), errors.Is(err, domain.ErrCapacityExceeded), errors.Is(err, domain.ErrExpired):
		return http.StatusConflict, "business_conflict", err.Error()
	case errors.Is(err, domain.ErrInvalidTransition):
		return http.StatusConflict, "invalid_transition", err.Error()
	default:
		return http.StatusInternalServerError, "internal_error", "internal server error"
	}
}

func parsePage(r *http.Request) repository.PageRequest {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	return repository.PageRequest{Limit: limit, Offset: offset, Sort: r.URL.Query().Get("sort"), Desc: r.URL.Query().Get("desc") == "true"}
}

func parseTimeQuery(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, domain.FieldError{Field: "time", Message: "must use RFC3339"}
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
