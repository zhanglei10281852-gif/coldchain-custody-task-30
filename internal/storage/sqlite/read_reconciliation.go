package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
)

func (q *queries) GetShipmentReconciliation(ctx context.Context, shipmentID string) (domain.ShipmentReconciliation, error) {
	shipment, err := q.GetShipment(ctx, shipmentID)
	if err != nil {
		return domain.ShipmentReconciliation{}, err
	}
	var report domain.ShipmentReconciliation
	report.ShipmentID = shipment.ID
	report.ShipmentState = shipment.State
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM shipment_items WHERE shipment_id = ?`, shipmentID).Scan(&report.ExpectedBatchCount); err != nil {
		return domain.ShipmentReconciliation{}, translateError("count shipment batches", err)
	}
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM shipment_items si JOIN sample_batches b ON b.id = si.batch_id
        WHERE si.shipment_id = ? AND b.state IN ('received', 'released', 'destroyed', 'quarantined')`, shipmentID).Scan(&report.ReceivedBatchCount); err != nil {
		return domain.ShipmentReconciliation{}, translateError("count received batches", err)
	}
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM shipment_items si JOIN sample_batches b ON b.id = si.batch_id
        WHERE si.shipment_id = ? AND b.state = 'released'`, shipmentID).Scan(&report.ReleasedBatchCount); err != nil {
		return domain.ShipmentReconciliation{}, translateError("count released batches", err)
	}
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM shipment_items si JOIN sample_batches b ON b.id = si.batch_id
        WHERE si.shipment_id = ? AND b.state = 'destroyed'`, shipmentID).Scan(&report.DestroyedBatchCount); err != nil {
		return domain.ShipmentReconciliation{}, translateError("count destroyed batches", err)
	}
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM shipment_items si JOIN sample_batches b ON b.id = si.batch_id
        WHERE si.shipment_id = ? AND b.state = 'quarantined'`, shipmentID).Scan(&report.QuarantinedCount); err != nil {
		return domain.ShipmentReconciliation{}, translateError("count quarantined batches", err)
	}
	var pending int
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM custody_handoffs WHERE shipment_id = ? AND status = 'pending'`, shipmentID).Scan(&pending); err != nil {
		return domain.ShipmentReconciliation{}, translateError("count pending handoffs", err)
	}
	report.PendingHandoff = pending > 0
	var open int
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM excursions WHERE shipment_id = ? AND status IN ('open', 'reviewing')`, shipmentID).Scan(&open); err != nil {
		return domain.ShipmentReconciliation{}, translateError("count open excursions", err)
	}
	report.OpenExcursion = open > 0
	var lastReading sql.NullString
	if err := q.q.QueryRowContext(ctx, `SELECT MAX(recorded_at) FROM temperature_readings WHERE shipment_id = ?`, shipmentID).Scan(&lastReading); err != nil {
		return domain.ShipmentReconciliation{}, translateError("get last reading", err)
	}
	if lastReading.Valid {
		parsed, err := parseTime(lastReading.String)
		if err != nil {
			return domain.ShipmentReconciliation{}, err
		}
		report.LastReadingAt = &parsed
	}
	report.Evaluate()
	return report.Clone(), nil
}

func (q *queries) latestReadingAt(ctx context.Context, shipmentID string) (time.Time, error) {
	var raw string
	if err := q.q.QueryRowContext(ctx, `SELECT recorded_at FROM temperature_readings WHERE shipment_id = ? ORDER BY recorded_at DESC LIMIT 1`, shipmentID).Scan(&raw); err != nil {
		return time.Time{}, translateError("get latest reading", err)
	}
	parsed, err := parseTime(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse latest reading: %w", err)
	}
	return parsed, nil
}
