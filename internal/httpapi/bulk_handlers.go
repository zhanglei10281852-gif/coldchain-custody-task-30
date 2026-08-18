package httpapi

import (
	"net/http"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
)

type bulkRegisterRequest struct {
	Batches []registerSampleRequest `json:"batches"`
}

func (s *Server) bulkRegisterSamples(w http.ResponseWriter, r *http.Request) {
	var input bulkRegisterRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	batches := make([]domain.SampleBatch, 0, len(input.Batches))
	for _, item := range input.Batches {
		batches = append(batches, domain.SampleBatch{StudyID: item.StudyID, OriginSiteID: item.OriginSiteID, ExternalRef: item.ExternalRef, SpecimenType: item.SpecimenType, VialCount: item.VialCount, VolumeMilliLit: item.VolumeMilliLit, ExpiresAt: item.ExpiresAt})
	}
	result, err := s.services.Catalog.BulkRegisterSamples(r.Context(), batches)
	if err != nil {
		writeError(w, r, err)
		return
	}
	status := http.StatusCreated
	if result.Failed > 0 {
		status = http.StatusMultiStatus
	}
	writeJSON(w, status, result)
}

func (s *Server) startContainerCleaning(w http.ResponseWriter, r *http.Request) {
	container, err := s.services.Containers.StartCleaning(r.Context(), parseID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, container)
}

func (s *Server) completeContainerCleaning(w http.ResponseWriter, r *http.Request) {
	container, err := s.services.Containers.CompleteCleaning(r.Context(), parseID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, container)
}

func (s *Server) retireContainer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	container, err := s.services.Containers.Retire(r.Context(), parseID(r), input.Reason)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, container)
}
