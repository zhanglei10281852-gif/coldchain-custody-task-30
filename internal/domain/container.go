package domain

import (
	"strings"
	"time"
)

type ContainerState string

const (
	ContainerAvailable ContainerState = "available"
	ContainerReserved  ContainerState = "reserved"
	ContainerInTransit ContainerState = "in_transit"
	ContainerCleaning  ContainerState = "cleaning"
	ContainerRetired   ContainerState = "retired"
)

type Container struct {
	ID                 string         `json:"id"`
	SerialNumber       string         `json:"serial_number"`
	State              ContainerState `json:"state"`
	CapacityMilliLit   int            `json:"capacity_ml"`
	CalibrationDueAt   time.Time      `json:"calibration_due_at"`
	LastCleanedAt      time.Time      `json:"last_cleaned_at"`
	ReservedShipmentID string         `json:"reserved_shipment_id,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	Version            int64          `json:"version"`
}

func (c Container) Validate() error {
	if strings.TrimSpace(c.SerialNumber) == "" {
		return FieldError{Field: "serial_number", Message: "is required"}
	}
	if c.CapacityMilliLit < 100 || c.CapacityMilliLit > 1000000 {
		return FieldError{Field: "capacity_ml", Message: "outside supported range"}
	}
	if c.CalibrationDueAt.IsZero() || c.LastCleanedAt.IsZero() {
		return FieldError{Field: "container", Message: "calibration and cleaning timestamps are required"}
	}
	switch c.State {
	case ContainerAvailable, ContainerReserved, ContainerInTransit, ContainerCleaning, ContainerRetired:
		return nil
	default:
		return FieldError{Field: "container_state", Message: "is invalid"}
	}
}

func (c Container) EligibleFor(plannedStart time.Time, volume int) error {
	if c.State != ContainerAvailable {
		return ConflictError{Resource: "container", Reason: "not available"}
	}
	if !c.IsCalibratedAt(plannedStart) {
		return ConflictError{Resource: "container", Reason: "calibration expires before dispatch"}
	}
	if c.CapacityMilliLit < volume {
		return ErrCapacityExceeded
	}
	return nil
}

func (c Container) IsCalibratedAt(at time.Time) bool {
	return c.CalibrationDueAt.After(at) && !c.LastCleanedAt.After(at)
}

func (c Container) NeedsCleaning(at time.Time) bool {
	return c.LastCleanedAt.IsZero() || c.LastCleanedAt.Before(at.Add(-72*time.Hour))
}

func (c *Container) StartCleaning(now time.Time) error {
	if c.State != ContainerAvailable {
		return TransitionError{Entity: "container", From: string(c.State), To: string(ContainerCleaning)}
	}
	c.State = ContainerCleaning
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *Container) CompleteCleaning(now time.Time) error {
	if c.State != ContainerCleaning {
		return TransitionError{Entity: "container", From: string(c.State), To: string(ContainerAvailable)}
	}
	c.State = ContainerAvailable
	c.LastCleanedAt = now.UTC()
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *Container) Retire(now time.Time) error {
	if c.State != ContainerAvailable && c.State != ContainerCleaning {
		return ConflictError{Resource: "container", Reason: "active reservation must be completed before retirement"}
	}
	c.State = ContainerRetired
	c.ReservedShipmentID = ""
	c.UpdatedAt = now.UTC()
	return nil
}
