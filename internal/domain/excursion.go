package domain

import (
	"strings"
	"time"
)

type ExcursionStatus string

const (
	ExcursionOpen      ExcursionStatus = "open"
	ExcursionReviewing ExcursionStatus = "reviewing"
	ExcursionCleared   ExcursionStatus = "cleared"
	ExcursionRejected  ExcursionStatus = "rejected"
)

type TemperatureReading struct {
	ID          string       `json:"id"`
	ShipmentID  string       `json:"shipment_id"`
	SensorID    string       `json:"sensor_id"`
	Sequence    int64        `json:"sequence"`
	Temperature MilliCelsius `json:"temperature_millicelsius"`
	RecordedAt  time.Time    `json:"recorded_at"`
	ReceivedAt  time.Time    `json:"received_at"`
}

type Excursion struct {
	ID             string          `json:"id"`
	ShipmentID     string          `json:"shipment_id"`
	Status         ExcursionStatus `json:"status"`
	FirstReadingAt time.Time       `json:"first_reading_at"`
	LastReadingAt  time.Time       `json:"last_reading_at"`
	Minimum        MilliCelsius    `json:"minimum_millicelsius"`
	Maximum        MilliCelsius    `json:"maximum_millicelsius"`
	ReadingCount   int             `json:"reading_count"`
	ReviewDueAt    time.Time       `json:"review_due_at"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	Version        int64           `json:"version"`
}

type ReviewDecision struct {
	ID          string
	ExcursionID string
	Reviewer    string
	Decision    ExcursionStatus
	Rationale   string
	CreatedAt   time.Time
}

func (s ExcursionStatus) IsResolved() bool {
	return s == ExcursionCleared || s == ExcursionRejected
}

func (s ExcursionStatus) IsOpen() bool { return s == ExcursionOpen || s == ExcursionReviewing }

func (r TemperatureReading) Validate() error {
	if strings.TrimSpace(r.ShipmentID) == "" || strings.TrimSpace(r.SensorID) == "" {
		return FieldError{Field: "reading", Message: "shipment and sensor are required"}
	}
	if r.Sequence < 1 || r.RecordedAt.IsZero() {
		return FieldError{Field: "reading", Message: "sequence and recorded_at are required"}
	}
	return nil
}

func (e *Excursion) Include(reading TemperatureReading, now time.Time) {
	if e.ReadingCount == 0 || reading.RecordedAt.Before(e.FirstReadingAt) {
		e.FirstReadingAt = reading.RecordedAt
	}
	if e.ReadingCount == 0 || reading.RecordedAt.After(e.LastReadingAt) {
		e.LastReadingAt = reading.RecordedAt
	}
	if e.ReadingCount == 0 || reading.Temperature < e.Minimum {
		e.Minimum = reading.Temperature
	}
	if e.ReadingCount == 0 || reading.Temperature > e.Maximum {
		e.Maximum = reading.Temperature
	}
	e.ReadingCount++
	e.UpdatedAt = now.UTC()
}

func (e *Excursion) StartReview(now time.Time) error {
	if e.Status != ExcursionOpen {
		return TransitionError{Entity: "excursion", From: string(e.Status), To: string(ExcursionReviewing)}
	}
	e.Status = ExcursionReviewing
	e.UpdatedAt = now.UTC()
	return nil
}

func (e *Excursion) Decide(decision ExcursionStatus, now time.Time) error {
	if e.Status != ExcursionOpen && e.Status != ExcursionReviewing {
		return TransitionError{Entity: "excursion", From: string(e.Status), To: string(decision)}
	}
	if decision != ExcursionCleared && decision != ExcursionRejected {
		return FieldError{Field: "decision", Message: "must be cleared or rejected"}
	}
	e.Status = decision
	e.UpdatedAt = now.UTC()
	return nil
}
