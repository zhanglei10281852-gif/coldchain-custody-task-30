package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
)

type scanner interface {
	Scan(dest ...any) error
}

func (q *queries) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	row := q.q.QueryRowContext(ctx, userSelect+` WHERE email = ? COLLATE NOCASE`, strings.TrimSpace(email))
	user, err := scanUser(row)
	return user, translateError("get user by email", err)
}

func (q *queries) GetUser(ctx context.Context, id string) (domain.User, error) {
	user, err := scanUser(q.q.QueryRowContext(ctx, userSelect+` WHERE id = ?`, id))
	return user, translateError("get user", err)
}

const userSelect = `SELECT id, email, display_name, password_hash, role, status, version, created_at, updated_at FROM users`

func scanUser(row scanner) (domain.User, error) {
	var user domain.User
	var role, status, createdAt, updatedAt string
	if err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &role, &status, &user.Version, &createdAt, &updatedAt); err != nil {
		return domain.User{}, err
	}
	var err error
	if user.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.User{}, err
	}
	if user.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.User{}, err
	}
	user.Role = domain.Role(role)
	user.Status = domain.UserStatus(status)
	return user, nil
}

func (q *queries) GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	row := q.q.QueryRowContext(ctx, `SELECT id, user_id, token_hash, expires_at, created_at, revoked_at FROM sessions WHERE token_hash = ?`, tokenHash)
	var session domain.Session
	var expiresAt, createdAt string
	var revokedAt sql.NullString
	if err := row.Scan(&session.ID, &session.UserID, &session.TokenHash, &expiresAt, &createdAt, &revokedAt); err != nil {
		return domain.Session{}, translateError("get session", err)
	}
	var err error
	if session.ExpiresAt, err = parseTime(expiresAt); err != nil {
		return domain.Session{}, err
	}
	if session.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Session{}, err
	}
	if session.RevokedAt, err = parseNullableTime(revokedAt); err != nil {
		return domain.Session{}, err
	}
	return session, nil
}

func (q *queries) GetStudy(ctx context.Context, id string) (domain.Study, error) {
	row := q.q.QueryRowContext(ctx, `SELECT id, code, name, status, minimum_millicelsius, maximum_millicelsius,
        max_transit_seconds, review_deadline_seconds, business_timezone, version, created_at, updated_at
        FROM studies WHERE id = ?`, id)
	var study domain.Study
	var status, createdAt, updatedAt string
	var maxTransitSeconds, reviewDeadlineSeconds int64
	if err := row.Scan(&study.ID, &study.Code, &study.Name, &status, &study.Temperature.Minimum,
		&study.Temperature.Maximum, &maxTransitSeconds, &reviewDeadlineSeconds, &study.BusinessTimezone,
		&study.Version, &createdAt, &updatedAt); err != nil {
		return domain.Study{}, translateError("get study", err)
	}
	study.Status = domain.StudyStatus(status)
	study.MaxTransit = durationSeconds(maxTransitSeconds)
	study.ReviewDeadline = durationSeconds(reviewDeadlineSeconds)
	var err error
	if study.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Study{}, err
	}
	if study.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Study{}, err
	}
	return study, nil
}

func (q *queries) GetSite(ctx context.Context, id string) (domain.Site, error) {
	row := q.q.QueryRowContext(ctx, `SELECT id, code, name, timezone, status, daily_limit, cutoff_hour, version, created_at, updated_at FROM sites WHERE id = ?`, id)
	var site domain.Site
	var status, createdAt, updatedAt string
	if err := row.Scan(&site.ID, &site.Code, &site.Name, &site.Timezone, &status, &site.DailyLimit, &site.CutoffHour, &site.Version, &createdAt, &updatedAt); err != nil {
		return domain.Site{}, translateError("get site", err)
	}
	site.Status = domain.SiteStatus(status)
	var err error
	if site.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Site{}, err
	}
	if site.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Site{}, err
	}
	return site, nil
}

func (q *queries) GetSampleBatch(ctx context.Context, id string) (domain.SampleBatch, error) {
	batch, err := scanSample(q.q.QueryRowContext(ctx, sampleSelect+` WHERE id = ?`, id))
	return batch, translateError("get sample batch", err)
}

const sampleSelect = `SELECT id, study_id, origin_site_id, external_ref, specimen_type, vial_count, volume_ml,
    state, expires_at, COALESCE(shipment_id, ''), quarantine_note, version, created_at, updated_at FROM sample_batches`

func scanSample(row scanner) (domain.SampleBatch, error) {
	var batch domain.SampleBatch
	var state, expiresAt, createdAt, updatedAt string
	if err := row.Scan(&batch.ID, &batch.StudyID, &batch.OriginSiteID, &batch.ExternalRef, &batch.SpecimenType,
		&batch.VialCount, &batch.VolumeMilliLit, &state, &expiresAt, &batch.ShipmentID, &batch.QuarantineNote,
		&batch.Version, &createdAt, &updatedAt); err != nil {
		return domain.SampleBatch{}, err
	}
	batch.State = domain.SampleState(state)
	var err error
	if batch.ExpiresAt, err = parseTime(expiresAt); err != nil {
		return domain.SampleBatch{}, err
	}
	if batch.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.SampleBatch{}, err
	}
	if batch.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.SampleBatch{}, err
	}
	return batch, nil
}

func (q *queries) GetContainer(ctx context.Context, id string) (domain.Container, error) {
	row := q.q.QueryRowContext(ctx, `SELECT id, serial_number, state, capacity_ml, calibration_due_at, last_cleaned_at,
        COALESCE(reserved_shipment_id, ''), version, created_at, updated_at FROM containers WHERE id = ?`, id)
	container, err := scanContainer(row)
	return container, translateError("get container", err)
}

func scanContainer(row scanner) (domain.Container, error) {
	var container domain.Container
	var state, calibrationDueAt, lastCleanedAt, createdAt, updatedAt string
	if err := row.Scan(&container.ID, &container.SerialNumber, &state, &container.CapacityMilliLit,
		&calibrationDueAt, &lastCleanedAt, &container.ReservedShipmentID, &container.Version, &createdAt, &updatedAt); err != nil {
		return domain.Container{}, err
	}
	container.State = domain.ContainerState(state)
	var err error
	if container.CalibrationDueAt, err = parseTime(calibrationDueAt); err != nil {
		return domain.Container{}, err
	}
	if container.LastCleanedAt, err = parseTime(lastCleanedAt); err != nil {
		return domain.Container{}, err
	}
	if container.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Container{}, err
	}
	if container.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Container{}, err
	}
	return container, nil
}

func (q *queries) GetShipment(ctx context.Context, id string) (domain.Shipment, error) {
	shipment, err := scanShipment(q.q.QueryRowContext(ctx, shipmentSelect+` WHERE id = ?`, id))
	return shipment, translateError("get shipment", err)
}

const shipmentSelect = `SELECT id, study_id, origin_site_id, destination_site_id, container_id, reference, state,
    planned_dispatch_at, expected_arrival_at, dispatched_at, arrived_at, closed_at, total_volume_ml, version, created_at, updated_at FROM shipments`

func scanShipment(row scanner) (domain.Shipment, error) {
	var shipment domain.Shipment
	var state, plannedDispatchAt, expectedArrivalAt, createdAt, updatedAt string
	var dispatchedAt, arrivedAt, closedAt sql.NullString
	if err := row.Scan(&shipment.ID, &shipment.StudyID, &shipment.OriginSiteID, &shipment.DestinationSiteID,
		&shipment.ContainerID, &shipment.Reference, &state, &plannedDispatchAt, &expectedArrivalAt,
		&dispatchedAt, &arrivedAt, &closedAt, &shipment.TotalVolumeMilliLit, &shipment.Version,
		&createdAt, &updatedAt); err != nil {
		return domain.Shipment{}, err
	}
	shipment.State = domain.ShipmentState(state)
	var err error
	if shipment.PlannedDispatchAt, err = parseTime(plannedDispatchAt); err != nil {
		return domain.Shipment{}, err
	}
	if shipment.ExpectedArrivalAt, err = parseTime(expectedArrivalAt); err != nil {
		return domain.Shipment{}, err
	}
	if shipment.DispatchedAt, err = parseNullableTime(dispatchedAt); err != nil {
		return domain.Shipment{}, err
	}
	if shipment.ArrivedAt, err = parseNullableTime(arrivedAt); err != nil {
		return domain.Shipment{}, err
	}
	if shipment.ClosedAt, err = parseNullableTime(closedAt); err != nil {
		return domain.Shipment{}, err
	}
	if shipment.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Shipment{}, err
	}
	if shipment.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Shipment{}, err
	}
	return shipment, nil
}

func (q *queries) ListShipmentItems(ctx context.Context, shipmentID string) ([]domain.SampleBatch, error) {
	rows, err := q.q.QueryContext(ctx, `SELECT sample_batches.id, sample_batches.study_id, sample_batches.origin_site_id,
        sample_batches.external_ref, sample_batches.specimen_type, sample_batches.vial_count, sample_batches.volume_ml,
        sample_batches.state, sample_batches.expires_at, COALESCE(sample_batches.shipment_id, ''), sample_batches.quarantine_note,
        sample_batches.version, sample_batches.created_at, sample_batches.updated_at
        FROM sample_batches JOIN shipment_items si ON si.batch_id = sample_batches.id
        WHERE si.shipment_id = ? ORDER BY si.added_at, sample_batches.id`, shipmentID)
	if err != nil {
		return nil, translateError("list shipment items", err)
	}
	defer rows.Close()
	items := make([]domain.SampleBatch, 0)
	for rows.Next() {
		batch, err := scanSample(rows)
		if err != nil {
			return nil, fmt.Errorf("scan shipment item: %w", err)
		}
		items = append(items, batch.Clone())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shipment items: %w", err)
	}
	return items, nil
}

func decodeMetadata(raw string) (map[string]string, error) {
	metadata := make(map[string]string)
	if raw == "" || raw == "{}" {
		return metadata, nil
	}
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, fmt.Errorf("decode audit metadata: %w", err)
	}
	return metadata, nil
}
