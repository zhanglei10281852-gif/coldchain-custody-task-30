package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/identity"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

type TelemetryService struct{ dependencies }

type RecordReadingInput struct {
	ShipmentID  string
	SensorID    string
	Sequence    int64
	Temperature domain.MilliCelsius
	RecordedAt  time.Time
}

func (s *TelemetryService) RecordReading(ctx context.Context, input RecordReadingInput) (domain.TemperatureReading, *domain.Excursion, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleCourier, domain.RoleOperations); err != nil {
		return domain.TemperatureReading{}, nil, err
	}
	now := s.clock.Now()
	reading := domain.TemperatureReading{ID: identity.New("read"), ShipmentID: input.ShipmentID, SensorID: input.SensorID, Sequence: input.Sequence, Temperature: input.Temperature, RecordedAt: input.RecordedAt.UTC(), ReceivedAt: now}
	if err := reading.Validate(); err != nil {
		return domain.TemperatureReading{}, nil, err
	}
	var excursion *domain.Excursion
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		shipment, err := tx.GetShipment(ctx, input.ShipmentID)
		if err != nil {
			return err
		}
		study, err := tx.GetStudy(ctx, shipment.StudyID)
		if err != nil {
			return err
		}
		if shipment.State != domain.ShipmentDispatched && shipment.State != domain.ShipmentArrived {
			return domain.ConflictError{Resource: "shipment", Reason: "temperature readings require active transit"}
		}
		if err := tx.InsertReading(ctx, reading); err != nil {
			return err
		}
		if study.Temperature.Contains(reading.Temperature) {
			return s.audit.Record(ctx, tx, "temperature_recorded", "shipment", shipment.ID, "success", map[string]string{"in_range": "true"})
		}
		active, err := tx.GetActiveExcursion(ctx, shipment.ID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		if errors.Is(err, domain.ErrNotFound) {
			active = domain.Excursion{ID: identity.New("exc"), ShipmentID: shipment.ID, Status: domain.ExcursionOpen, ReviewDueAt: now.Add(study.ReviewDeadline), Version: 1, CreatedAt: now, UpdatedAt: now}
			active.Include(reading, now)
			if err := tx.InsertExcursion(ctx, active); err != nil {
				return err
			}
		} else {
			before := active.Version
			active.Include(reading, now)
			if err := tx.UpdateExcursion(ctx, active, before); err != nil {
				return err
			}
		}
		items, err := tx.ListShipmentItems(ctx, shipment.ID)
		if err != nil {
			return err
		}
		for _, batch := range items {
			if batch.State == domain.SampleInTransit || batch.State == domain.SampleReceived {
				batch.State = domain.SampleQuarantined
				batch.QuarantineNote = fmt.Sprintf("temperature excursion %s", active.ID)
				batch.UpdatedAt = now
				if err := tx.UpdateSampleBatch(ctx, batch, batch.Version); err != nil {
					return err
				}
			}
		}
		payload := []byte(active.ID)
		if err := tx.InsertJob(ctx, domain.OutboxJob{ID: identity.New("job"), Kind: "excursion_review", AggregateID: active.ID, Payload: payload, Status: domain.JobPending, MaxAttempts: 5, AvailableAt: now, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		excursion = &active
		return s.audit.Record(ctx, tx, "temperature_excursion_opened", "excursion", active.ID, "success", map[string]string{"shipment_id": shipment.ID})
	})
	return reading, excursion, err
}
