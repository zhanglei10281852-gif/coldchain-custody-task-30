package worker

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/clock"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/storage/sqlite"
)

func workerFixture(t *testing.T) (*Worker, *sqlite.Store, context.Context, time.Time) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	fixed := clock.NewFixed(now)
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "worker.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	return New(store, fixed, time.Second, 20, logger), store, ctx, now
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestRunOnceExpiresHandoffsAndCompletesJobs(t *testing.T) {
	worker, store, ctx, now := workerFixture(t)
	minimum, _ := domain.TemperatureFromCelsius(2)
	maximum, _ := domain.TemperatureFromCelsius(8)
	rangeValue, _ := domain.NewTemperatureRange(minimum, maximum)
	study := domain.Study{ID: "study_worker", Code: "WORKER", Name: "Worker", Status: domain.StudyActive, Temperature: rangeValue, MaxTransit: time.Hour, ReviewDeadline: time.Hour, BusinessTimezone: "UTC", Version: 1, CreatedAt: now, UpdatedAt: now}
	origin := domain.Site{ID: "origin_worker", Code: "ORIGIN", Name: "Origin", Timezone: "UTC", Status: domain.SiteActive, DailyLimit: 10, CutoffHour: 0, Version: 1, CreatedAt: now, UpdatedAt: now}
	destination := domain.Site{ID: "dest_worker", Code: "DEST", Name: "Destination", Timezone: "UTC", Status: domain.SiteActive, DailyLimit: 10, CutoffHour: 0, Version: 1, CreatedAt: now, UpdatedAt: now}
	container := domain.Container{ID: "box_worker", SerialNumber: "BOX-W", State: domain.ContainerInTransit, CapacityMilliLit: 1000, CalibrationDueAt: now.Add(time.Hour), LastCleanedAt: now, ReservedShipmentID: "ship_worker", Version: 1, CreatedAt: now, UpdatedAt: now}
	shipment := domain.Shipment{ID: "ship_worker", StudyID: study.ID, OriginSiteID: origin.ID, DestinationSiteID: destination.ID, ContainerID: container.ID, Reference: "SHIP-W", State: domain.ShipmentDispatched, PlannedDispatchAt: now, ExpectedArrivalAt: now.Add(time.Hour), TotalVolumeMilliLit: 1, Version: 1, CreatedAt: now, UpdatedAt: now}
	handoff := domain.CustodyHandoff{ID: "handoff_worker", ShipmentID: shipment.ID, FromCustodian: "from", ToCustodian: "to", Location: "dock", Status: domain.HandoffPending, ExpiresAt: now.Add(-time.Minute), Version: 1, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	job := domain.OutboxJob{ID: "job_worker", Kind: "shipment_planned", AggregateID: shipment.ID, Payload: []byte(`{"id":"ship_worker"}`), Status: domain.JobPending, MaxAttempts: 3, AvailableAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	if err := store.WithTx(ctx, func(tx repository.Tx) error {
		for _, entity := range []any{study, origin, destination, container, shipment, handoff, job} {
			switch value := entity.(type) {
			case domain.Study:
				if err := tx.InsertStudy(ctx, value); err != nil {
					return err
				}
			case domain.Site:
				if err := tx.InsertSite(ctx, value); err != nil {
					return err
				}
			case domain.Container:
				if err := tx.InsertContainer(ctx, value); err != nil {
					return err
				}
			case domain.Shipment:
				if err := tx.InsertShipment(ctx, value); err != nil {
					return err
				}
			case domain.CustodyHandoff:
				if err := tx.InsertHandoff(ctx, value); err != nil {
					return err
				}
			case domain.OutboxJob:
				if err := tx.InsertJob(ctx, value); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Read(ctx, func(reader repository.Reader) error {
		handoff, err := reader.GetHandoff(ctx, handoff.ID)
		if err != nil {
			return err
		}
		if handoff.Status != domain.HandoffExpired {
			t.Fatalf("handoff = %+v", handoff)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRunOnceHonorsCancellation(t *testing.T) {
	worker, _, _, _ := workerFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := worker.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v", err)
	}
}
