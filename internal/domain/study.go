package domain

import (
	"strings"
	"time"
)

type StudyStatus string

const (
	StudyDraft  StudyStatus = "draft"
	StudyActive StudyStatus = "active"
	StudyClosed StudyStatus = "closed"
)

type Study struct {
	ID               string           `json:"id"`
	Code             string           `json:"code"`
	Name             string           `json:"name"`
	Status           StudyStatus      `json:"status"`
	Temperature      TemperatureRange `json:"temperature"`
	MaxTransit       time.Duration    `json:"max_transit"`
	ReviewDeadline   time.Duration    `json:"review_deadline"`
	BusinessTimezone string           `json:"business_timezone"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	Version          int64            `json:"version"`
}

func (s Study) Validate() error {
	if strings.TrimSpace(s.Code) == "" {
		return FieldError{Field: "code", Message: "is required"}
	}
	if strings.TrimSpace(s.Name) == "" {
		return FieldError{Field: "name", Message: "is required"}
	}
	if err := s.Temperature.Validate(); err != nil {
		return err
	}
	if s.MaxTransit <= 0 || s.MaxTransit > 14*24*time.Hour {
		return FieldError{Field: "max_transit", Message: "must be between zero and fourteen days"}
	}
	if s.ReviewDeadline <= 0 || s.ReviewDeadline > 7*24*time.Hour {
		return FieldError{Field: "review_deadline", Message: "must be between zero and seven days"}
	}
	if _, err := time.LoadLocation(s.BusinessTimezone); err != nil {
		return FieldError{Field: "business_timezone", Message: "is invalid"}
	}
	switch s.Status {
	case StudyDraft, StudyActive, StudyClosed:
		return nil
	default:
		return FieldError{Field: "status", Message: "is invalid"}
	}
}

func (s Study) CanAcceptShipments() bool { return s.Status == StudyActive }

func (s Study) TransitWithinLimit(dispatchAt, arrivalAt time.Time) bool {
	return arrivalAt.After(dispatchAt) && arrivalAt.Sub(dispatchAt) <= s.MaxTransit
}

func (s Study) IsClosed() bool { return s.Status == StudyClosed }
