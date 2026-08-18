package domain

import "time"

type ShipmentReconciliation struct {
	ShipmentID          string        `json:"shipment_id"`
	ShipmentState       ShipmentState `json:"shipment_state"`
	ExpectedBatchCount  int           `json:"expected_batch_count"`
	ReceivedBatchCount  int           `json:"received_batch_count"`
	ReleasedBatchCount  int           `json:"released_batch_count"`
	DestroyedBatchCount int           `json:"destroyed_batch_count"`
	QuarantinedCount    int           `json:"quarantined_count"`
	PendingHandoff      bool          `json:"pending_handoff"`
	OpenExcursion       bool          `json:"open_excursion"`
	LastReadingAt       *time.Time    `json:"last_reading_at,omitempty"`
	Complete            bool          `json:"complete"`
	Blockers            []string      `json:"blockers"`
}

func (r ShipmentReconciliation) Clone() ShipmentReconciliation {
	clone := r
	clone.Blockers = append([]string(nil), r.Blockers...)
	if r.LastReadingAt != nil {
		value := *r.LastReadingAt
		clone.LastReadingAt = &value
	}
	return clone
}

func (r *ShipmentReconciliation) Evaluate() {
	r.Blockers = r.Blockers[:0]
	if r.PendingHandoff {
		r.Blockers = append(r.Blockers, "pending custody handoff")
	}
	if r.OpenExcursion {
		r.Blockers = append(r.Blockers, "open temperature excursion")
	}
	if r.QuarantinedCount > 0 {
		r.Blockers = append(r.Blockers, "quarantined samples require review")
	}
	if r.ExpectedBatchCount == 0 {
		r.Blockers = append(r.Blockers, "shipment has no sample batches")
	}
	if r.ReceivedBatchCount < r.ExpectedBatchCount && r.ShipmentState == ShipmentArrived {
		r.Blockers = append(r.Blockers, "not all samples are received")
	}
	r.Complete = len(r.Blockers) == 0 && (r.ShipmentState == ShipmentClosed || r.ShipmentState == ShipmentArrived)
}
