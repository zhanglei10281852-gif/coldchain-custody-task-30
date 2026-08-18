package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

func (q *queries) InsertUser(ctx context.Context, user domain.User) error {
	if err := user.Validate(); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `INSERT INTO users(id, email, display_name, password_hash, role, status, version, created_at, updated_at)
        VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`, user.ID, user.Email, user.DisplayName, user.PasswordHash, user.Role,
		user.Status, user.Version, formatTime(user.CreatedAt), formatTime(user.UpdatedAt))
	return translateError("insert user", err)
}

func (q *queries) InsertSession(ctx context.Context, session domain.Session) error {
	_, err := q.q.ExecContext(ctx, `INSERT INTO sessions(id, user_id, token_hash, expires_at, created_at, revoked_at)
        VALUES(?, ?, ?, ?, ?, NULL)`, session.ID, session.UserID, session.TokenHash, formatTime(session.ExpiresAt), formatTime(session.CreatedAt))
	return translateError("insert session", err)
}

func (q *queries) RevokeSession(ctx context.Context, sessionID string, revokedAt time.Time) error {
	result, err := q.q.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, formatTime(revokedAt), sessionID)
	if err != nil {
		return translateError("revoke session", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke session rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("revoke session: %w", domain.ErrNotFound)
	}
	return nil
}

func (q *queries) InsertStudy(ctx context.Context, study domain.Study) error {
	if err := study.Validate(); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `INSERT INTO studies(id, code, name, status, minimum_millicelsius, maximum_millicelsius,
        max_transit_seconds, review_deadline_seconds, business_timezone, version, created_at, updated_at)
        VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, study.ID, study.Code, study.Name, study.Status,
		study.Temperature.Minimum, study.Temperature.Maximum, int64(study.MaxTransit/time.Second),
		int64(study.ReviewDeadline/time.Second), study.BusinessTimezone, study.Version,
		formatTime(study.CreatedAt), formatTime(study.UpdatedAt))
	return translateError("insert study", err)
}

func (q *queries) UpdateStudy(ctx context.Context, study domain.Study, expectedVersion int64) error {
	if err := study.Validate(); err != nil {
		return err
	}
	result, err := q.q.ExecContext(ctx, `UPDATE studies SET status = ?, version = version + 1, updated_at = ? WHERE id = ? AND version = ?`, study.Status, formatTime(study.UpdatedAt), study.ID, expectedVersion)
	if err != nil {
		return translateError("update study", err)
	}
	return expectVersion(result, "update study")
}

func (q *queries) InsertSite(ctx context.Context, site domain.Site) error {
	if err := site.Validate(); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `INSERT INTO sites(id, code, name, timezone, status, daily_limit, cutoff_hour, version, created_at, updated_at)
        VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, site.ID, site.Code, site.Name, site.Timezone, site.Status,
		site.DailyLimit, site.CutoffHour, site.Version, formatTime(site.CreatedAt), formatTime(site.UpdatedAt))
	return translateError("insert site", err)
}

func (q *queries) InsertSampleBatch(ctx context.Context, batch domain.SampleBatch) error {
	if err := batch.Validate(); err != nil {
		return err
	}
	shipmentID := nullableString(batch.ShipmentID)
	_, err := q.q.ExecContext(ctx, `INSERT INTO sample_batches(id, study_id, origin_site_id, external_ref, specimen_type,
        vial_count, volume_ml, state, expires_at, shipment_id, quarantine_note, version, created_at, updated_at)
        VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, batch.ID, batch.StudyID, batch.OriginSiteID,
		batch.ExternalRef, batch.SpecimenType, batch.VialCount, batch.VolumeMilliLit, batch.State,
		formatTime(batch.ExpiresAt), shipmentID, batch.QuarantineNote, batch.Version,
		formatTime(batch.CreatedAt), formatTime(batch.UpdatedAt))
	return translateError("insert sample batch", err)
}

func (q *queries) UpdateSampleBatch(ctx context.Context, batch domain.SampleBatch, expectedVersion int64) error {
	if err := batch.Validate(); err != nil {
		return err
	}
	result, err := q.q.ExecContext(ctx, `UPDATE sample_batches SET state = ?, shipment_id = ?, quarantine_note = ?,
        expires_at = ?, version = version + 1, updated_at = ? WHERE id = ? AND version = ?`, batch.State,
		nullableString(batch.ShipmentID), batch.QuarantineNote, formatTime(batch.ExpiresAt), formatTime(batch.UpdatedAt),
		batch.ID, expectedVersion)
	if err != nil {
		return translateError("update sample batch", err)
	}
	return expectVersion(result, "update sample batch")
}

func (q *queries) InsertContainer(ctx context.Context, container domain.Container) error {
	if err := container.Validate(); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `INSERT INTO containers(id, serial_number, state, capacity_ml, calibration_due_at,
        last_cleaned_at, reserved_shipment_id, version, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		container.ID, container.SerialNumber, container.State, container.CapacityMilliLit,
		formatTime(container.CalibrationDueAt), formatTime(container.LastCleanedAt), nullableString(container.ReservedShipmentID),
		container.Version, formatTime(container.CreatedAt), formatTime(container.UpdatedAt))
	return translateError("insert container", err)
}

func (q *queries) UpdateContainer(ctx context.Context, container domain.Container, expectedVersion int64) error {
	if err := container.Validate(); err != nil {
		return err
	}
	result, err := q.q.ExecContext(ctx, `UPDATE containers SET state = ?, capacity_ml = ?, calibration_due_at = ?,
        last_cleaned_at = ?, reserved_shipment_id = ?, version = version + 1, updated_at = ? WHERE id = ? AND version = ?`,
		container.State, container.CapacityMilliLit, formatTime(container.CalibrationDueAt), formatTime(container.LastCleanedAt),
		nullableString(container.ReservedShipmentID), formatTime(container.UpdatedAt), container.ID, expectedVersion)
	if err != nil {
		return translateError("update container", err)
	}
	return expectVersion(result, "update container")
}

func (q *queries) InsertShipment(ctx context.Context, shipment domain.Shipment) error {
	if err := shipment.Validate(); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `INSERT INTO shipments(id, study_id, origin_site_id, destination_site_id, container_id,
        reference, state, planned_dispatch_at, expected_arrival_at, dispatched_at, arrived_at, closed_at,
        total_volume_ml, version, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		shipment.ID, shipment.StudyID, shipment.OriginSiteID, shipment.DestinationSiteID, shipment.ContainerID,
		shipment.Reference, shipment.State, formatTime(shipment.PlannedDispatchAt), formatTime(shipment.ExpectedArrivalAt),
		nullableTime(shipment.DispatchedAt), nullableTime(shipment.ArrivedAt), nullableTime(shipment.ClosedAt),
		shipment.TotalVolumeMilliLit, shipment.Version, formatTime(shipment.CreatedAt), formatTime(shipment.UpdatedAt))
	return translateError("insert shipment", err)
}

func (q *queries) UpdateShipment(ctx context.Context, shipment domain.Shipment, expectedVersion int64) error {
	if err := shipment.Validate(); err != nil {
		return err
	}
	result, err := q.q.ExecContext(ctx, `UPDATE shipments SET state = ?, dispatched_at = ?, arrived_at = ?, closed_at = ?,
        version = version + 1, updated_at = ? WHERE id = ? AND version = ?`, shipment.State,
		nullableTime(shipment.DispatchedAt), nullableTime(shipment.ArrivedAt), nullableTime(shipment.ClosedAt),
		formatTime(shipment.UpdatedAt), shipment.ID, expectedVersion)
	if err != nil {
		return translateError("update shipment", err)
	}
	return expectVersion(result, "update shipment")
}

func (q *queries) InsertShipmentItem(ctx context.Context, item domain.ShipmentItem) error {
	_, err := q.q.ExecContext(ctx, `INSERT INTO shipment_items(shipment_id, batch_id, added_at) VALUES(?, ?, ?)`,
		item.ShipmentID, item.BatchID, formatTime(item.AddedAt))
	return translateError("insert shipment item", err)
}

func (q *queries) InsertHandoff(ctx context.Context, handoff domain.CustodyHandoff) error {
	if err := handoff.Validate(); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `INSERT INTO custody_handoffs(id, shipment_id, from_custodian, to_custodian,
        location, status, expires_at, resolved_at, resolution_note, version, created_at, updated_at)
        VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, handoff.ID, handoff.ShipmentID, handoff.FromCustodian,
		handoff.ToCustodian, handoff.Location, handoff.Status, formatTime(handoff.ExpiresAt), nullableTime(handoff.ResolvedAt),
		handoff.ResolutionNote, handoff.Version, formatTime(handoff.CreatedAt), formatTime(handoff.UpdatedAt))
	return translateError("insert handoff", err)
}

func (q *queries) UpdateHandoff(ctx context.Context, handoff domain.CustodyHandoff, expectedVersion int64) error {
	if err := handoff.Validate(); err != nil {
		return err
	}
	result, err := q.q.ExecContext(ctx, `UPDATE custody_handoffs SET status = ?, resolved_at = ?, resolution_note = ?,
        version = version + 1, updated_at = ? WHERE id = ? AND version = ?`, handoff.Status,
		nullableTime(handoff.ResolvedAt), handoff.ResolutionNote, formatTime(handoff.UpdatedAt), handoff.ID, expectedVersion)
	if err != nil {
		return translateError("update handoff", err)
	}
	return expectVersion(result, "update handoff")
}

func (q *queries) InsertReading(ctx context.Context, reading domain.TemperatureReading) error {
	if err := reading.Validate(); err != nil {
		return err
	}
	_, err := q.q.ExecContext(ctx, `INSERT INTO temperature_readings(id, shipment_id, sensor_id, sequence,
        temperature_millicelsius, recorded_at, received_at) VALUES(?, ?, ?, ?, ?, ?, ?)`, reading.ID,
		reading.ShipmentID, reading.SensorID, reading.Sequence, reading.Temperature,
		formatTime(reading.RecordedAt), formatTime(reading.ReceivedAt))
	return translateError("insert temperature reading", err)
}

func (q *queries) InsertExcursion(ctx context.Context, excursion domain.Excursion) error {
	_, err := q.q.ExecContext(ctx, `INSERT INTO excursions(id, shipment_id, status, first_reading_at, last_reading_at,
        minimum_millicelsius, maximum_millicelsius, reading_count, review_due_at, version, created_at, updated_at)
        VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, excursion.ID, excursion.ShipmentID, excursion.Status,
		formatTime(excursion.FirstReadingAt), formatTime(excursion.LastReadingAt), excursion.Minimum, excursion.Maximum,
		excursion.ReadingCount, formatTime(excursion.ReviewDueAt), excursion.Version,
		formatTime(excursion.CreatedAt), formatTime(excursion.UpdatedAt))
	return translateError("insert excursion", err)
}

func (q *queries) UpdateExcursion(ctx context.Context, excursion domain.Excursion, expectedVersion int64) error {
	result, err := q.q.ExecContext(ctx, `UPDATE excursions SET status = ?, first_reading_at = ?, last_reading_at = ?,
        minimum_millicelsius = ?, maximum_millicelsius = ?, reading_count = ?, review_due_at = ?,
        version = version + 1, updated_at = ? WHERE id = ? AND version = ?`, excursion.Status,
		formatTime(excursion.FirstReadingAt), formatTime(excursion.LastReadingAt), excursion.Minimum, excursion.Maximum,
		excursion.ReadingCount, formatTime(excursion.ReviewDueAt), formatTime(excursion.UpdatedAt), excursion.ID, expectedVersion)
	if err != nil {
		return translateError("update excursion", err)
	}
	return expectVersion(result, "update excursion")
}

func (q *queries) InsertReviewDecision(ctx context.Context, decision domain.ReviewDecision) error {
	_, err := q.q.ExecContext(ctx, `INSERT INTO review_decisions(id, excursion_id, reviewer, decision, rationale, created_at)
        VALUES(?, ?, ?, ?, ?, ?)`, decision.ID, decision.ExcursionID, decision.Reviewer, decision.Decision,
		decision.Rationale, formatTime(decision.CreatedAt))
	return translateError("insert review decision", err)
}

func (q *queries) InsertAuditEvent(ctx context.Context, event domain.AuditEvent) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}
	_, err = q.q.ExecContext(ctx, `INSERT INTO audit_events(id, request_id, actor, action, entity_type, entity_id,
        outcome, metadata_json, created_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.ID, event.RequestID,
		event.Actor, event.Action, event.EntityType, event.EntityID, event.Outcome, string(metadata), formatTime(event.CreatedAt))
	return translateError("insert audit event", err)
}

func (q *queries) PutIdempotency(ctx context.Context, record repository.IdempotencyRecord) error {
	_, err := q.q.ExecContext(ctx, `INSERT INTO idempotency_records(scope, idempotency_key, request_hash,
        response_code, response_body, expires_at, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)`, record.Scope,
		record.Key, record.RequestHash, record.ResponseCode, append([]byte(nil), record.ResponseBody...),
		formatTime(record.ExpiresAt), formatTime(record.CreatedAt))
	return translateError("put idempotency record", err)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
