package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/service"
)

type planShipmentRequest struct {
	StudyID           string    `json:"study_id"`
	OriginSiteID      string    `json:"origin_site_id"`
	DestinationSiteID string    `json:"destination_site_id"`
	ContainerID       string    `json:"container_id"`
	Reference         string    `json:"reference"`
	BatchIDs          []string  `json:"batch_ids"`
	PlannedDispatchAt time.Time `json:"planned_dispatch_at"`
	ExpectedArrivalAt time.Time `json:"expected_arrival_at"`
}

func (s *Server) planShipment(w http.ResponseWriter, r *http.Request) {
	var input planShipmentRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	shipment, err := s.services.Planning.PlanShipment(r.Context(), service.PlanShipmentInput{StudyID: input.StudyID, OriginSiteID: input.OriginSiteID, DestinationSiteID: input.DestinationSiteID, ContainerID: input.ContainerID, Reference: input.Reference, BatchIDs: append([]string(nil), input.BatchIDs...), PlannedDispatchAt: input.PlannedDispatchAt, ExpectedArrivalAt: input.ExpectedArrivalAt, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, shipment)
}

func (s *Server) getShipment(w http.ResponseWriter, r *http.Request) {
	shipment, items, err := s.services.Query.Shipment(r.Context(), parseID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shipment": shipment, "items": items})
}

func (s *Server) reconcileShipment(w http.ResponseWriter, r *http.Request) {
	report, err := s.services.Query.ReconcileShipment(r.Context(), parseID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) listShipments(w http.ResponseWriter, r *http.Request) {
	from, err := parseTimeQuery(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	to, err := parseTimeQuery(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	page, err := s.services.Query.Shipments(r.Context(), repository.ShipmentFilter{Page: parsePage(r), StudyID: r.URL.Query().Get("study_id"), OriginSiteID: r.URL.Query().Get("origin_site_id"), DestinationID: r.URL.Query().Get("destination_site_id"), State: domain.ShipmentState(r.URL.Query().Get("state")), From: from, To: to})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) packShipment(w http.ResponseWriter, r *http.Request) {
	s.writeShipmentTransition(w, r, s.services.Planning.PackShipment)
}
func (s *Server) dispatchShipment(w http.ResponseWriter, r *http.Request) {
	s.writeShipmentTransition(w, r, s.services.Planning.DispatchShipment)
}
func (s *Server) arriveShipment(w http.ResponseWriter, r *http.Request) {
	s.writeShipmentTransition(w, r, s.services.Planning.ArriveShipment)
}
func (s *Server) closeShipment(w http.ResponseWriter, r *http.Request) {
	s.writeShipmentTransition(w, r, s.services.Planning.CloseShipment)
}

func (s *Server) writeShipmentTransition(w http.ResponseWriter, r *http.Request, transition func(context.Context, string) (domain.Shipment, error)) {
	shipment, err := transition(r.Context(), parseID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, shipment)
}

func (s *Server) cancelShipment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Note string `json:"note"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	shipment, err := s.services.Planning.CancelShipment(r.Context(), parseID(r), input.Note)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, shipment)
}
