package domain

import (
	"strings"
	"time"
)

type HandoffStatus string

const (
	HandoffPending  HandoffStatus = "pending"
	HandoffAccepted HandoffStatus = "accepted"
	HandoffRejected HandoffStatus = "rejected"
	HandoffExpired  HandoffStatus = "expired"
)

func (s HandoffStatus) IsResolved() bool {
	return s == HandoffAccepted || s == HandoffRejected || s == HandoffExpired
}

func (s HandoffStatus) IsPending() bool { return s == HandoffPending }

type CustodyHandoff struct {
	ID             string        `json:"id"`
	ShipmentID     string        `json:"shipment_id"`
	FromCustodian  string        `json:"from_custodian"`
	ToCustodian    string        `json:"to_custodian"`
	Location       string        `json:"location"`
	Status         HandoffStatus `json:"status"`
	ExpiresAt      time.Time     `json:"expires_at"`
	ResolvedAt     *time.Time    `json:"resolved_at,omitempty"`
	ResolutionNote string        `json:"resolution_note,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	Version        int64         `json:"version"`
}

func (h CustodyHandoff) Validate() error {
	if strings.TrimSpace(h.ShipmentID) == "" || strings.TrimSpace(h.FromCustodian) == "" || strings.TrimSpace(h.ToCustodian) == "" {
		return FieldError{Field: "handoff", Message: "shipment and custodians are required"}
	}
	if h.FromCustodian == h.ToCustodian {
		return FieldError{Field: "to_custodian", Message: "must differ from sender"}
	}
	if strings.TrimSpace(h.Location) == "" || h.ExpiresAt.IsZero() {
		return FieldError{Field: "handoff", Message: "location and expiry are required"}
	}
	switch h.Status {
	case HandoffPending, HandoffAccepted, HandoffRejected, HandoffExpired:
		return nil
	default:
		return FieldError{Field: "handoff_status", Message: "is invalid"}
	}
}

func (h *CustodyHandoff) Resolve(status HandoffStatus, note string, now time.Time) error {
	if h.Status != HandoffPending {
		return TransitionError{Entity: "custody_handoff", From: string(h.Status), To: string(status)}
	}
	if now.After(h.ExpiresAt) && status != HandoffExpired {
		return ErrExpired
	}
	if status != HandoffAccepted && status != HandoffRejected && status != HandoffExpired {
		return FieldError{Field: "handoff_status", Message: "unsupported resolution"}
	}
	now = now.UTC()
	h.Status = status
	h.ResolutionNote = strings.TrimSpace(note)
	h.ResolvedAt = &now
	h.UpdatedAt = now
	return nil
}
