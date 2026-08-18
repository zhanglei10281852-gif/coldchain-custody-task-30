package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

func (q *queries) GetPendingHandoff(ctx context.Context, shipmentID string) (domain.CustodyHandoff, error) {
	handoff, err := scanHandoff(q.q.QueryRowContext(ctx, handoffSelect+` WHERE shipment_id = ? AND status = 'pending'`, shipmentID))
	return handoff, translateError("get pending handoff", err)
}

func (q *queries) GetHandoff(ctx context.Context, id string) (domain.CustodyHandoff, error) {
	handoff, err := scanHandoff(q.q.QueryRowContext(ctx, handoffSelect+` WHERE id = ?`, id))
	return handoff, translateError("get handoff", err)
}

const handoffSelect = `SELECT id, shipment_id, from_custodian, to_custodian, location, status, expires_at,
    resolved_at, resolution_note, version, created_at, updated_at FROM custody_handoffs`

func scanHandoff(row scanner) (domain.CustodyHandoff, error) {
	var handoff domain.CustodyHandoff
	var status, expiresAt, createdAt, updatedAt string
	var resolvedAt sql.NullString
	if err := row.Scan(&handoff.ID, &handoff.ShipmentID, &handoff.FromCustodian, &handoff.ToCustodian,
		&handoff.Location, &status, &expiresAt, &resolvedAt, &handoff.ResolutionNote,
		&handoff.Version, &createdAt, &updatedAt); err != nil {
		return domain.CustodyHandoff{}, err
	}
	handoff.Status = domain.HandoffStatus(status)
	var err error
	if handoff.ExpiresAt, err = parseTime(expiresAt); err != nil {
		return domain.CustodyHandoff{}, err
	}
	if handoff.ResolvedAt, err = parseNullableTime(resolvedAt); err != nil {
		return domain.CustodyHandoff{}, err
	}
	if handoff.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.CustodyHandoff{}, err
	}
	if handoff.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.CustodyHandoff{}, err
	}
	return handoff, nil
}

func (q *queries) GetActiveExcursion(ctx context.Context, shipmentID string) (domain.Excursion, error) {
	excursion, err := scanExcursion(q.q.QueryRowContext(ctx, excursionSelect+` WHERE shipment_id = ? AND status IN ('open', 'reviewing')`, shipmentID))
	return excursion, translateError("get active excursion", err)
}

func (q *queries) GetExcursion(ctx context.Context, id string) (domain.Excursion, error) {
	excursion, err := scanExcursion(q.q.QueryRowContext(ctx, excursionSelect+` WHERE id = ?`, id))
	return excursion, translateError("get excursion", err)
}

const excursionSelect = `SELECT id, shipment_id, status, first_reading_at, last_reading_at,
    minimum_millicelsius, maximum_millicelsius, reading_count, review_due_at, version, created_at, updated_at FROM excursions`

func scanExcursion(row scanner) (domain.Excursion, error) {
	var excursion domain.Excursion
	var status, firstReadingAt, lastReadingAt, reviewDueAt, createdAt, updatedAt string
	if err := row.Scan(&excursion.ID, &excursion.ShipmentID, &status, &firstReadingAt, &lastReadingAt,
		&excursion.Minimum, &excursion.Maximum, &excursion.ReadingCount, &reviewDueAt,
		&excursion.Version, &createdAt, &updatedAt); err != nil {
		return domain.Excursion{}, err
	}
	excursion.Status = domain.ExcursionStatus(status)
	var err error
	if excursion.FirstReadingAt, err = parseTime(firstReadingAt); err != nil {
		return domain.Excursion{}, err
	}
	if excursion.LastReadingAt, err = parseTime(lastReadingAt); err != nil {
		return domain.Excursion{}, err
	}
	if excursion.ReviewDueAt, err = parseTime(reviewDueAt); err != nil {
		return domain.Excursion{}, err
	}
	if excursion.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Excursion{}, err
	}
	if excursion.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Excursion{}, err
	}
	return excursion, nil
}

func (q *queries) GetIdempotency(ctx context.Context, scope, key string) (repository.IdempotencyRecord, error) {
	row := q.q.QueryRowContext(ctx, `SELECT scope, idempotency_key, request_hash, response_code, response_body, expires_at, created_at
        FROM idempotency_records WHERE scope = ? AND idempotency_key = ?`, scope, key)
	var record repository.IdempotencyRecord
	var expiresAt, createdAt string
	if err := row.Scan(&record.Scope, &record.Key, &record.RequestHash, &record.ResponseCode, &record.ResponseBody, &expiresAt, &createdAt); err != nil {
		return repository.IdempotencyRecord{}, translateError("get idempotency record", err)
	}
	var err error
	if record.ExpiresAt, err = parseTime(expiresAt); err != nil {
		return repository.IdempotencyRecord{}, err
	}
	if record.CreatedAt, err = parseTime(createdAt); err != nil {
		return repository.IdempotencyRecord{}, err
	}
	record.ResponseBody = append([]byte(nil), record.ResponseBody...)
	return record, nil
}

func (q *queries) CountSiteShipmentsForBusinessDay(ctx context.Context, siteID, businessDay string) (int, error) {
	var count int
	err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM shipments
        WHERE origin_site_id = ? AND substr(planned_dispatch_at, 1, 10) = ? AND state != 'cancelled'`, siteID, businessDay).Scan(&count)
	if err != nil {
		return 0, translateError("count site shipments", err)
	}
	return count, nil
}

func (q *queries) ListShipments(ctx context.Context, filter repository.ShipmentFilter) (repository.ShipmentPage, error) {
	page := filter.Page.Normalize(200)
	where, args := buildShipmentWhere(filter)
	var total int
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM shipments`+where, args...).Scan(&total); err != nil {
		return repository.ShipmentPage{}, translateError("count shipments", err)
	}
	sortColumn := shipmentSortColumn(page.Sort)
	direction := " ASC"
	if page.Desc {
		direction = " DESC"
	}
	query := shipmentSelect + where + ` ORDER BY ` + sortColumn + direction + `, id ASC LIMIT ? OFFSET ?`
	rows, err := q.q.QueryContext(ctx, query, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return repository.ShipmentPage{}, translateError("list shipments", err)
	}
	defer rows.Close()
	items := make([]domain.Shipment, 0, page.Limit)
	for rows.Next() {
		shipment, err := scanShipment(rows)
		if err != nil {
			return repository.ShipmentPage{}, fmt.Errorf("scan shipment: %w", err)
		}
		items = append(items, shipment)
	}
	if err := rows.Err(); err != nil {
		return repository.ShipmentPage{}, fmt.Errorf("iterate shipments: %w", err)
	}
	return repository.ShipmentPage{Items: items, Total: total}, nil
}

func buildShipmentWhere(filter repository.ShipmentFilter) (string, []any) {
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	appendStringFilter := func(column, value string) {
		if value != "" {
			clauses = append(clauses, column+" = ?")
			args = append(args, value)
		}
	}
	appendStringFilter("study_id", filter.StudyID)
	appendStringFilter("origin_site_id", filter.OriginSiteID)
	appendStringFilter("destination_site_id", filter.DestinationID)
	appendStringFilter("state", string(filter.State))
	if filter.From != nil {
		clauses = append(clauses, "planned_dispatch_at >= ?")
		args = append(args, formatTime(*filter.From))
	}
	if filter.To != nil {
		clauses = append(clauses, "planned_dispatch_at < ?")
		args = append(args, formatTime(*filter.To))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func shipmentSortColumn(value string) string {
	switch value {
	case "expected_arrival_at":
		return "expected_arrival_at"
	case "updated_at":
		return "updated_at"
	case "reference":
		return "reference"
	default:
		return "planned_dispatch_at"
	}
}

func (q *queries) ListSamples(ctx context.Context, filter repository.SampleFilter) (repository.SamplePage, error) {
	page := filter.Page.Normalize(200)
	where, args := buildSampleWhere(filter)
	var total int
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM sample_batches`+where, args...).Scan(&total); err != nil {
		return repository.SamplePage{}, translateError("count samples", err)
	}
	query := sampleSelect + where + ` ORDER BY expires_at ASC, id ASC LIMIT ? OFFSET ?`
	rows, err := q.q.QueryContext(ctx, query, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return repository.SamplePage{}, translateError("list samples", err)
	}
	defer rows.Close()
	items := make([]domain.SampleBatch, 0, page.Limit)
	for rows.Next() {
		batch, err := scanSample(rows)
		if err != nil {
			return repository.SamplePage{}, fmt.Errorf("scan sample: %w", err)
		}
		items = append(items, batch.Clone())
	}
	if err := rows.Err(); err != nil {
		return repository.SamplePage{}, fmt.Errorf("iterate samples: %w", err)
	}
	return repository.SamplePage{Items: items, Total: total}, nil
}

func buildSampleWhere(filter repository.SampleFilter) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	values := []struct{ column, value string }{
		{"study_id", filter.StudyID}, {"origin_site_id", filter.SiteID}, {"shipment_id", filter.ShipmentID}, {"state", string(filter.State)},
	}
	for _, item := range values {
		if item.value != "" {
			clauses = append(clauses, item.column+" = ?")
			args = append(args, item.value)
		}
	}
	if filter.ExpiresBy != nil {
		clauses = append(clauses, "expires_at <= ?")
		args = append(args, formatTime(*filter.ExpiresBy))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (q *queries) ListExcursions(ctx context.Context, filter repository.ExcursionFilter) (repository.ExcursionPage, error) {
	page := filter.Page.Normalize(200)
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if filter.ShipmentID != "" {
		clauses = append(clauses, "shipment_id = ?")
		args = append(args, filter.ShipmentID)
	}
	if filter.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.DueBefore != nil {
		clauses = append(clauses, "review_due_at <= ?")
		args = append(args, formatTime(*filter.DueBefore))
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	var total int
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM excursions`+where, args...).Scan(&total); err != nil {
		return repository.ExcursionPage{}, translateError("count excursions", err)
	}
	rows, err := q.q.QueryContext(ctx, excursionSelect+where+` ORDER BY review_due_at ASC, id ASC LIMIT ? OFFSET ?`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return repository.ExcursionPage{}, translateError("list excursions", err)
	}
	defer rows.Close()
	items := make([]domain.Excursion, 0, page.Limit)
	for rows.Next() {
		excursion, err := scanExcursion(rows)
		if err != nil {
			return repository.ExcursionPage{}, fmt.Errorf("scan excursion: %w", err)
		}
		items = append(items, excursion)
	}
	if err := rows.Err(); err != nil {
		return repository.ExcursionPage{}, fmt.Errorf("iterate excursions: %w", err)
	}
	return repository.ExcursionPage{Items: items, Total: total}, nil
}

func (q *queries) ListAuditEvents(ctx context.Context, filter repository.AuditFilter) (repository.AuditPage, error) {
	page := filter.Page.Normalize(500)
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	values := []struct{ column, value string }{
		{"entity_type", filter.EntityType}, {"entity_id", filter.EntityID}, {"actor", filter.Actor}, {"request_id", filter.RequestID},
	}
	for _, item := range values {
		if item.value != "" {
			clauses = append(clauses, item.column+" = ?")
			args = append(args, item.value)
		}
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	var total int
	if err := q.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events`+where, args...).Scan(&total); err != nil {
		return repository.AuditPage{}, translateError("count audit events", err)
	}
	rows, err := q.q.QueryContext(ctx, `SELECT id, request_id, actor, action, entity_type, entity_id, outcome, metadata_json, created_at
        FROM audit_events`+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return repository.AuditPage{}, translateError("list audit events", err)
	}
	defer rows.Close()
	items := make([]domain.AuditEvent, 0, page.Limit)
	for rows.Next() {
		var event domain.AuditEvent
		var metadataJSON, createdAt string
		if err := rows.Scan(&event.ID, &event.RequestID, &event.Actor, &event.Action, &event.EntityType,
			&event.EntityID, &event.Outcome, &metadataJSON, &createdAt); err != nil {
			return repository.AuditPage{}, fmt.Errorf("scan audit event: %w", err)
		}
		metadata, err := decodeMetadata(metadataJSON)
		if err != nil {
			return repository.AuditPage{}, err
		}
		event.Metadata = metadata
		if event.CreatedAt, err = parseTime(createdAt); err != nil {
			return repository.AuditPage{}, err
		}
		items = append(items, event.Clone())
	}
	if err := rows.Err(); err != nil {
		return repository.AuditPage{}, fmt.Errorf("iterate audit events: %w", err)
	}
	return repository.AuditPage{Items: items, Total: total}, nil
}

func beginningOfUTCDate(day string) (time.Time, error) {
	return time.Parse("2006-01-02", day)
}
