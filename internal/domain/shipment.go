package domain

import (
	"strings"
	"time"
)

type ShipmentState string

const (
	ShipmentPlanned    ShipmentState = "planned"
	ShipmentPacked     ShipmentState = "packed"
	ShipmentDispatched ShipmentState = "dispatched"
	ShipmentArrived    ShipmentState = "arrived"
	ShipmentClosed     ShipmentState = "closed"
	ShipmentCancelled  ShipmentState = "cancelled"
)

type Shipment struct {
	ID                  string        `json:"id"`
	StudyID             string        `json:"study_id"`
	OriginSiteID        string        `json:"origin_site_id"`
	DestinationSiteID   string        `json:"destination_site_id"`
	ContainerID         string        `json:"container_id"`
	Reference           string        `json:"reference"`
	State               ShipmentState `json:"state"`
	PlannedDispatchAt   time.Time     `json:"planned_dispatch_at"`
	ExpectedArrivalAt   time.Time     `json:"expected_arrival_at"`
	DispatchedAt        *time.Time    `json:"dispatched_at,omitempty"`
	ArrivedAt           *time.Time    `json:"arrived_at,omitempty"`
	ClosedAt            *time.Time    `json:"closed_at,omitempty"`
	TotalVolumeMilliLit int           `json:"total_volume_ml"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
	Version             int64         `json:"version"`
}

type ShipmentItem struct {
	ShipmentID string
	BatchID    string
	AddedAt    time.Time
}

func (s Shipment) Validate() error {
	if strings.TrimSpace(s.StudyID) == "" || strings.TrimSpace(s.OriginSiteID) == "" || strings.TrimSpace(s.DestinationSiteID) == "" {
		return FieldError{Field: "shipment", Message: "study, origin and destination are required"}
	}
	if s.OriginSiteID == s.DestinationSiteID {
		return FieldError{Field: "destination_site_id", Message: "must differ from origin"}
	}
	if strings.TrimSpace(s.Reference) == "" || strings.TrimSpace(s.ContainerID) == "" {
		return FieldError{Field: "shipment", Message: "reference and container are required"}
	}
	if !s.ExpectedArrivalAt.After(s.PlannedDispatchAt) {
		return FieldError{Field: "expected_arrival_at", Message: "must be after dispatch"}
	}
	if s.TotalVolumeMilliLit < 1 {
		return FieldError{Field: "total_volume_ml", Message: "must be positive"}
	}
	return validateShipmentState(s.State)
}

func validateShipmentState(state ShipmentState) error {
	switch state {
	case ShipmentPlanned, ShipmentPacked, ShipmentDispatched, ShipmentArrived, ShipmentClosed, ShipmentCancelled:
		return nil
	default:
		return FieldError{Field: "shipment_state", Message: "is invalid"}
	}
}

func (s ShipmentState) IsTerminal() bool {
	return s == ShipmentClosed || s == ShipmentCancelled
}

func (s *Shipment) Transition(to ShipmentState, now time.Time) error {
	allowed := map[ShipmentState]map[ShipmentState]bool{
		ShipmentPlanned:    {ShipmentPacked: true, ShipmentCancelled: true},
		ShipmentPacked:     {ShipmentDispatched: true, ShipmentCancelled: true},
		ShipmentDispatched: {ShipmentArrived: true},
		ShipmentArrived:    {ShipmentClosed: true},
	}
	if !allowed[s.State][to] {
		return TransitionError{Entity: "shipment", From: string(s.State), To: string(to)}
	}
	now = now.UTC()
	switch to {
	case ShipmentDispatched:
		s.DispatchedAt = &now
	case ShipmentArrived:
		s.ArrivedAt = &now
	case ShipmentClosed:
		s.ClosedAt = &now
	}
	s.State = to
	s.UpdatedAt = now
	return nil
}
