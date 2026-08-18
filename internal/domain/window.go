package domain

import (
	"strings"
	"time"
)

type TransitWindow struct {
	DispatchAt time.Time
	ArrivalAt  time.Time
}

func (w TransitWindow) Duration() time.Duration {
	return w.ArrivalAt.Sub(w.DispatchAt)
}

func (w TransitWindow) Validate(study Study, batches []SampleBatch, now time.Time) error {
	if w.DispatchAt.IsZero() || w.ArrivalAt.IsZero() {
		return FieldError{Field: "transit_window", Message: "dispatch and arrival are required"}
	}
	if !w.ArrivalAt.After(w.DispatchAt) {
		return FieldError{Field: "arrival_at", Message: "must be after dispatch"}
	}
	if w.Duration() > study.MaxTransit {
		return ConflictError{Resource: "shipment", Reason: "transit exceeds study limit"}
	}
	if w.DispatchAt.Before(now.Add(-15 * time.Minute)) {
		return ConflictError{Resource: "shipment", Reason: "dispatch window is already closed"}
	}
	for _, batch := range batches {
		if !batch.ExpiresAt.After(w.ArrivalAt) {
			return ConflictError{Resource: "sample_batch", Reason: "sample expires before expected arrival"}
		}
	}
	return nil
}

func ValidateRoute(origin, destination Site) error {
	if strings.TrimSpace(origin.ID) == "" || strings.TrimSpace(destination.ID) == "" {
		return FieldError{Field: "route", Message: "origin and destination are required"}
	}
	if origin.ID == destination.ID {
		return FieldError{Field: "route", Message: "origin and destination must differ"}
	}
	if origin.Status != SiteActive || destination.Status != SiteActive {
		return ConflictError{Resource: "route", Reason: "both sites must be active"}
	}
	return nil
}
