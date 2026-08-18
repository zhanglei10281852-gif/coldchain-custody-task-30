package httpapi

import (
	"net/http"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
)

type createStudyRequest struct {
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	MinimumCelsius   float64 `json:"minimum_celsius"`
	MaximumCelsius   float64 `json:"maximum_celsius"`
	MaxTransitHours  int     `json:"max_transit_hours"`
	ReviewHours      int     `json:"review_hours"`
	BusinessTimezone string  `json:"business_timezone"`
}

func (s *Server) createStudy(w http.ResponseWriter, r *http.Request) {
	var input createStudyRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	minimum, err := domain.TemperatureFromCelsius(input.MinimumCelsius)
	if err != nil {
		writeError(w, r, err)
		return
	}
	maximum, err := domain.TemperatureFromCelsius(input.MaximumCelsius)
	if err != nil {
		writeError(w, r, err)
		return
	}
	rangeValue, err := domain.NewTemperatureRange(minimum, maximum)
	if err != nil {
		writeError(w, r, err)
		return
	}
	study, err := s.services.Catalog.CreateStudy(r.Context(), domain.Study{Code: input.Code, Name: input.Name, Temperature: rangeValue, MaxTransit: time.Duration(input.MaxTransitHours) * time.Hour, ReviewDeadline: time.Duration(input.ReviewHours) * time.Hour, BusinessTimezone: input.BusinessTimezone})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, study)
}

func (s *Server) activateStudy(w http.ResponseWriter, r *http.Request) {
	study, err := s.services.Catalog.ActivateStudy(r.Context(), parseID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, study)
}

type createSiteRequest struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Timezone   string `json:"timezone"`
	DailyLimit int    `json:"daily_limit"`
	CutoffHour int    `json:"cutoff_hour"`
}

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	var input createSiteRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	site, err := s.services.Catalog.CreateSite(r.Context(), domain.Site{Code: input.Code, Name: input.Name, Timezone: input.Timezone, DailyLimit: input.DailyLimit, CutoffHour: input.CutoffHour})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, site)
}

type createContainerRequest struct {
	SerialNumber     string    `json:"serial_number"`
	CapacityMilliLit int       `json:"capacity_ml"`
	CalibrationDueAt time.Time `json:"calibration_due_at"`
	LastCleanedAt    time.Time `json:"last_cleaned_at"`
}

func (s *Server) createContainer(w http.ResponseWriter, r *http.Request) {
	var input createContainerRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	container, err := s.services.Catalog.CreateContainer(r.Context(), domain.Container{SerialNumber: input.SerialNumber, CapacityMilliLit: input.CapacityMilliLit, CalibrationDueAt: input.CalibrationDueAt, LastCleanedAt: input.LastCleanedAt})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, container)
}

type registerSampleRequest struct {
	StudyID        string    `json:"study_id"`
	OriginSiteID   string    `json:"origin_site_id"`
	ExternalRef    string    `json:"external_ref"`
	SpecimenType   string    `json:"specimen_type"`
	VialCount      int       `json:"vial_count"`
	VolumeMilliLit int       `json:"volume_ml"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func (s *Server) registerSample(w http.ResponseWriter, r *http.Request) {
	var input registerSampleRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	batch, err := s.services.Catalog.RegisterSample(r.Context(), domain.SampleBatch{StudyID: input.StudyID, OriginSiteID: input.OriginSiteID, ExternalRef: input.ExternalRef, SpecimenType: input.SpecimenType, VialCount: input.VialCount, VolumeMilliLit: input.VolumeMilliLit, ExpiresAt: input.ExpiresAt})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, batch)
}

func (s *Server) readySample(w http.ResponseWriter, r *http.Request) {
	batch, err := s.services.Catalog.MarkSampleReady(r.Context(), parseID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}
