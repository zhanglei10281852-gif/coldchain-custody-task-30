package domain

import (
	"strings"
	"time"
)

type SampleState string

const (
	SampleRegistered  SampleState = "registered"
	SampleReady       SampleState = "ready"
	SampleReserved    SampleState = "reserved"
	SampleInTransit   SampleState = "in_transit"
	SampleReceived    SampleState = "received"
	SampleQuarantined SampleState = "quarantined"
	SampleReleased    SampleState = "released"
	SampleDestroyed   SampleState = "destroyed"
)

type SampleBatch struct {
	ID             string      `json:"id"`
	StudyID        string      `json:"study_id"`
	OriginSiteID   string      `json:"origin_site_id"`
	ExternalRef    string      `json:"external_ref"`
	SpecimenType   string      `json:"specimen_type"`
	VialCount      int         `json:"vial_count"`
	VolumeMilliLit int         `json:"volume_ml"`
	State          SampleState `json:"state"`
	ExpiresAt      time.Time   `json:"expires_at"`
	ShipmentID     string      `json:"shipment_id,omitempty"`
	QuarantineNote string      `json:"quarantine_note,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	Version        int64       `json:"version"`
}

func (b SampleBatch) Validate() error {
	if strings.TrimSpace(b.StudyID) == "" || strings.TrimSpace(b.OriginSiteID) == "" {
		return FieldError{Field: "sample_batch", Message: "study and origin site are required"}
	}
	if strings.TrimSpace(b.ExternalRef) == "" || strings.TrimSpace(b.SpecimenType) == "" {
		return FieldError{Field: "sample_batch", Message: "external reference and specimen type are required"}
	}
	if b.VialCount < 1 || b.VolumeMilliLit < 1 {
		return FieldError{Field: "sample_batch", Message: "vial count and volume must be positive"}
	}
	if b.ExpiresAt.IsZero() {
		return FieldError{Field: "expires_at", Message: "is required"}
	}
	return validateSampleState(b.State)
}

func validateSampleState(state SampleState) error {
	switch state {
	case SampleRegistered, SampleReady, SampleReserved, SampleInTransit, SampleReceived, SampleQuarantined, SampleReleased, SampleDestroyed:
		return nil
	default:
		return FieldError{Field: "sample_state", Message: "is invalid"}
	}
}

func (s SampleState) IsTerminal() bool {
	return s == SampleReleased || s == SampleDestroyed
}

func (b *SampleBatch) Transition(to SampleState, now time.Time) error {
	allowed := map[SampleState]map[SampleState]bool{
		SampleRegistered:  {SampleReady: true, SampleDestroyed: true},
		SampleReady:       {SampleReserved: true, SampleDestroyed: true},
		SampleReserved:    {SampleReady: true, SampleInTransit: true},
		SampleInTransit:   {SampleReceived: true, SampleQuarantined: true},
		SampleReceived:    {SampleReleased: true, SampleQuarantined: true},
		SampleQuarantined: {SampleReleased: true, SampleDestroyed: true},
	}
	if !allowed[b.State][to] {
		return TransitionError{Entity: "sample_batch", From: string(b.State), To: string(to)}
	}
	if !b.IsUsableAt(now) && to != SampleDestroyed && to != SampleQuarantined {
		return ConflictError{Resource: "sample_batch", Reason: "expired sample cannot advance"}
	}
	b.State = to
	b.UpdatedAt = now.UTC()
	return nil
}

func (b SampleBatch) Clone() SampleBatch { return b }

func (b SampleBatch) IsUsableAt(at time.Time) bool {
	return b.ExpiresAt.After(at) && b.State != SampleDestroyed
}
