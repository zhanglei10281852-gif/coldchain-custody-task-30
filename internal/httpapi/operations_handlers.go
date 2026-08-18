package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/service"
)

type createHandoffRequest struct {
	FromCustodian string `json:"from_custodian"`
	ToCustodian   string `json:"to_custodian"`
	Location      string `json:"location"`
}

func (s *Server) createHandoff(w http.ResponseWriter, r *http.Request) {
	var input createHandoffRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	handoff, err := s.services.Custody.CreateHandoff(r.Context(), service.CreateHandoffInput{ShipmentID: parseID(r), FromCustodian: input.FromCustodian, ToCustodian: input.ToCustodian, Location: input.Location})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, handoff)
}

type resolveHandoffRequest struct {
	Accepted bool   `json:"accepted"`
	Note     string `json:"note"`
}

func (s *Server) resolveHandoff(w http.ResponseWriter, r *http.Request) {
	var input resolveHandoffRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	handoff, err := s.services.Custody.ResolveHandoff(r.Context(), parseID(r), input.Accepted, input.Note)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, handoff)
}

type readingRequest struct {
	SensorID           string    `json:"sensor_id"`
	Sequence           int64     `json:"sequence"`
	TemperatureCelsius float64   `json:"temperature_celsius"`
	RecordedAt         time.Time `json:"recorded_at"`
}

func (s *Server) recordReading(w http.ResponseWriter, r *http.Request) {
	var input readingRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	temperature, err := domain.TemperatureFromCelsius(input.TemperatureCelsius)
	if err != nil {
		writeError(w, r, err)
		return
	}
	reading, excursion, err := s.services.Telemetry.RecordReading(r.Context(), service.RecordReadingInput{ShipmentID: parseID(r), SensorID: input.SensorID, Sequence: input.Sequence, Temperature: temperature, RecordedAt: input.RecordedAt})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reading": reading, "excursion": excursion})
}

func (s *Server) listExcursions(w http.ResponseWriter, r *http.Request) {
	dueBefore, err := parseTimeQuery(r.URL.Query().Get("due_before"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	page, err := s.services.Query.Excursions(r.Context(), repository.ExcursionFilter{Page: parsePage(r), ShipmentID: r.URL.Query().Get("shipment_id"), Status: domain.ExcursionStatus(r.URL.Query().Get("status")), DueBefore: dueBefore})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) startReview(w http.ResponseWriter, r *http.Request) {
	excursion, err := s.services.Review.StartReview(r.Context(), parseID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, excursion)
}

type decisionRequest struct {
	Decision  domain.ExcursionStatus `json:"decision"`
	Rationale string                 `json:"rationale"`
}

func (s *Server) decideExcursion(w http.ResponseWriter, r *http.Request) {
	var input decisionRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	excursion, err := s.services.Review.Decide(r.Context(), service.DecideInput{ExcursionID: parseID(r), Decision: input.Decision, Rationale: input.Rationale})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, excursion)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	page, err := s.services.Query.Audit(r.Context(), repository.AuditFilter{Page: parsePage(r), EntityType: r.URL.Query().Get("entity_type"), EntityID: r.URL.Query().Get("entity_id"), Actor: r.URL.Query().Get("actor"), RequestID: r.URL.Query().Get("request_id")})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.services.Query.OperationalSummary(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func queryInt(r *http.Request, key string) int {
	value, _ := strconv.Atoi(r.URL.Query().Get(key))
	return value
}
