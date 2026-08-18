package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

func testStore(t *testing.T) (*Store, context.Context, time.Time) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "coldchain.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	return store, ctx, now
}

func seedCatalog(t *testing.T, store *Store, ctx context.Context, now time.Time) (domain.Study, domain.Site, domain.Site, domain.Container, domain.SampleBatch) {
	t.Helper()
	minimum, _ := domain.TemperatureFromCelsius(2)
	maximum, _ := domain.TemperatureFromCelsius(8)
	rangeValue, _ := domain.NewTemperatureRange(minimum, maximum)
	study := domain.Study{ID: "study_1", Code: "STUDY-1", Name: "Cold study", Status: domain.StudyActive, Temperature: rangeValue, MaxTransit: 24 * time.Hour, ReviewDeadline: 4 * time.Hour, BusinessTimezone: "Asia/Shanghai", Version: 1, CreatedAt: now, UpdatedAt: now}
	origin := domain.Site{ID: "site_1", Code: "SITE-1", Name: "Origin", Timezone: "Asia/Shanghai", Status: domain.SiteActive, DailyLimit: 10, CutoffHour: 6, Version: 1, CreatedAt: now, UpdatedAt: now}
	destination := domain.Site{ID: "site_2", Code: "SITE-2", Name: "Destination", Timezone: "Asia/Shanghai", Status: domain.SiteActive, DailyLimit: 10, CutoffHour: 6, Version: 1, CreatedAt: now, UpdatedAt: now}
	container := domain.Container{ID: "box_1", SerialNumber: "BOX-1", State: domain.ContainerAvailable, CapacityMilliLit: 1000, CalibrationDueAt: now.Add(48 * time.Hour), LastCleanedAt: now, Version: 1, CreatedAt: now, UpdatedAt: now}
	batch := domain.SampleBatch{ID: "sample_1", StudyID: study.ID, OriginSiteID: origin.ID, ExternalRef: "EXT-1", SpecimenType: "plasma", VialCount: 2, VolumeMilliLit: 100, State: domain.SampleReady, ExpiresAt: now.Add(48 * time.Hour), Version: 1, CreatedAt: now, UpdatedAt: now}
	err := store.WithTx(ctx, func(tx repository.Tx) error {
		if err := tx.InsertStudy(ctx, study); err != nil {
			return err
		}
		if err := tx.InsertSite(ctx, origin); err != nil {
			return err
		}
		if err := tx.InsertSite(ctx, destination); err != nil {
			return err
		}
		if err := tx.InsertContainer(ctx, container); err != nil {
			return err
		}
		return tx.InsertSampleBatch(ctx, batch)
	})
	if err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	return study, origin, destination, container, batch
}

func TestOpenAppliesMigrationsAndEnablesForeignKeys(t *testing.T) {
	store, ctx, _ := testStore(t)
	if err := store.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	var tableCount int
	if err := store.Read(ctx, func(reader repository.Reader) error {
		_, err := reader.GetStudy(ctx, "missing")
		if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount < 15 {
		t.Fatalf("table count = %d, want at least 15", tableCount)
	}
	var foreignKeys int
	if err := store.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d", foreignKeys)
	}
}

func TestTransactionRollsBackAllEntities(t *testing.T) {
	store, ctx, now := testStore(t)
	minimum, _ := domain.TemperatureFromCelsius(2)
	maximum, _ := domain.TemperatureFromCelsius(8)
	rangeValue, _ := domain.NewTemperatureRange(minimum, maximum)
	study := domain.Study{ID: "study_roll", Code: "ROLL", Name: "Rollback", Status: domain.StudyActive, Temperature: rangeValue, MaxTransit: time.Hour, ReviewDeadline: time.Hour, BusinessTimezone: "UTC", Version: 1, CreatedAt: now, UpdatedAt: now}
	err := store.WithTx(ctx, func(tx repository.Tx) error {
		if err := tx.InsertStudy(ctx, study); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("rollback transaction returned nil")
	}
	if err := store.Read(ctx, func(reader repository.Reader) error {
		_, err := reader.GetStudy(ctx, study.ID)
		return err
	}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("study after rollback error = %v", err)
	}
}

func TestRepositoryReadsAndDeepCopiesSample(t *testing.T) {
	store, ctx, now := testStore(t)
	_, origin, _, _, batch := seedCatalog(t, store, ctx, now)
	got, err := storeReadSample(store, ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.OriginSiteID != origin.ID || got.State != domain.SampleReady {
		t.Fatalf("sample = %+v", got)
	}
	got.QuarantineNote = "local mutation"
	again, err := storeReadSample(store, ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.QuarantineNote != "" {
		t.Fatalf("stored sample was mutated: %+v", again)
	}
}

func storeReadSample(store *Store, ctx context.Context, id string) (domain.SampleBatch, error) {
	var result domain.SampleBatch
	err := store.Read(ctx, func(reader repository.Reader) error {
		var err error
		result, err = reader.GetSampleBatch(ctx, id)
		return err
	})
	return result, err
}

func TestOptimisticVersionRejectsStaleUpdate(t *testing.T) {
	store, ctx, now := testStore(t)
	_, _, _, _, batch := seedCatalog(t, store, ctx, now)
	first, err := storeReadSample(store, ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	second := first
	first.State = domain.SampleReserved
	first.UpdatedAt = now.Add(time.Minute)
	if err := store.WithTx(ctx, func(tx repository.Tx) error { return tx.UpdateSampleBatch(ctx, first, first.Version) }); err != nil {
		t.Fatal(err)
	}
	second.State = domain.SampleReserved
	second.UpdatedAt = now.Add(2 * time.Minute)
	err = store.WithTx(ctx, func(tx repository.Tx) error { return tx.UpdateSampleBatch(ctx, second, second.Version) })
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("stale update error = %v", err)
	}
}

func TestShipmentFilterPaginationAndOrdering(t *testing.T) {
	store, ctx, now := testStore(t)
	study, origin, destination, container, batch := seedCatalog(t, store, ctx, now)
	secondBatch := batch
	secondBatch.ID = "sample_2"
	secondBatch.ExternalRef = "EXT-2"
	if err := store.WithTx(ctx, func(tx repository.Tx) error { return tx.InsertSampleBatch(ctx, secondBatch) }); err != nil {
		t.Fatal(err)
	}
	shipments := []domain.Shipment{
		{ID: "ship_1", StudyID: study.ID, OriginSiteID: origin.ID, DestinationSiteID: destination.ID, ContainerID: container.ID, Reference: "REF-1", State: domain.ShipmentPlanned, PlannedDispatchAt: now.Add(time.Hour), ExpectedArrivalAt: now.Add(2 * time.Hour), TotalVolumeMilliLit: 100, Version: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "ship_2", StudyID: study.ID, OriginSiteID: origin.ID, DestinationSiteID: destination.ID, ContainerID: container.ID, Reference: "REF-2", State: domain.ShipmentPacked, PlannedDispatchAt: now.Add(2 * time.Hour), ExpectedArrivalAt: now.Add(3 * time.Hour), TotalVolumeMilliLit: 100, Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	if err := store.WithTx(ctx, func(tx repository.Tx) error {
		for _, shipment := range shipments {
			if err := tx.InsertShipment(ctx, shipment); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var page repository.ShipmentPage
	err := store.Read(ctx, func(reader repository.Reader) error {
		var err error
		page, err = reader.ListShipments(ctx, repository.ShipmentFilter{Page: repository.PageRequest{Limit: 1, Sort: "planned_dispatch_at"}, StudyID: study.ID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 1 || page.Items[0].ID != "ship_1" {
		t.Fatalf("page = %+v", page)
	}
}

func TestIdempotencyRecordCopiesResponse(t *testing.T) {
	store, ctx, now := testStore(t)
	record := repository.IdempotencyRecord{Scope: "scope", Key: "key", RequestHash: "hash", ResponseCode: 201, ResponseBody: []byte("body"), ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	if err := store.WithTx(ctx, func(tx repository.Tx) error { return tx.PutIdempotency(ctx, record) }); err != nil {
		t.Fatal(err)
	}
	var got repository.IdempotencyRecord
	if err := store.Read(ctx, func(reader repository.Reader) error {
		var err error
		got, err = reader.GetIdempotency(ctx, record.Scope, record.Key)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	got.ResponseBody[0] = 'B'
	var again repository.IdempotencyRecord
	if err := store.Read(ctx, func(reader repository.Reader) error {
		var err error
		again, err = reader.GetIdempotency(ctx, record.Scope, record.Key)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if string(again.ResponseBody) != "body" {
		t.Fatalf("response body = %q", again.ResponseBody)
	}
}

func TestOutboxClaimRetryAndCompletion(t *testing.T) {
	store, ctx, now := testStore(t)
	job := domain.OutboxJob{ID: "job_1", Kind: "shipment_planned", AggregateID: "ship_1", Payload: []byte("{}"), Status: domain.JobPending, MaxAttempts: 2, AvailableAt: now, CreatedAt: now, UpdatedAt: now}
	if err := store.WithTx(ctx, func(tx repository.Tx) error { return tx.InsertJob(ctx, job) }); err != nil {
		t.Fatal(err)
	}
	var claimed []domain.OutboxJob
	if err := store.WithTx(ctx, func(tx repository.Tx) error {
		var err error
		claimed, err = tx.ClaimJobs(ctx, now, 10)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].Attempts != 1 || claimed[0].Status != domain.JobRunning {
		t.Fatalf("claimed = %+v", claimed)
	}
	if err := store.WithTx(ctx, func(tx repository.Tx) error {
		return tx.RetryJob(ctx, job.ID, now.Add(time.Minute), "temporary", false)
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.WithTx(ctx, func(tx repository.Tx) error {
		jobs, err := tx.ClaimJobs(ctx, now.Add(2*time.Minute), 10)
		if err != nil || len(jobs) != 1 {
			return errors.New("job was not re-claimed")
		}
		return tx.CompleteJob(ctx, jobs[0].ID, now.Add(2*time.Minute))
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRestartRecoversPersistedRows(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "restart.db")
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, _, batch := seedCatalog(t, store, ctx, now)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := storeReadSample(reopened, ctx, batch.ID)
	if err != nil || got.ID != batch.ID {
		t.Fatalf("recovered sample = %+v, error = %v", got, err)
	}
}

func TestForeignKeyRejectsUnknownStudy(t *testing.T) {
	store, ctx, now := testStore(t)
	batch := domain.SampleBatch{ID: "orphan", StudyID: "missing", OriginSiteID: "missing", ExternalRef: "EXT", SpecimenType: "plasma", VialCount: 1, VolumeMilliLit: 1, State: domain.SampleRegistered, ExpiresAt: now.Add(time.Hour), Version: 1, CreatedAt: now, UpdatedAt: now}
	err := store.WithTx(ctx, func(tx repository.Tx) error { return tx.InsertSampleBatch(ctx, batch) })
	if err == nil {
		t.Fatal("orphan insert succeeded")
	}
}

func TestReconciliationCountsRelatedRows(t *testing.T) {
	store, ctx, now := testStore(t)
	study, origin, destination, container, batch := seedCatalog(t, store, ctx, now)
	shipment := domain.Shipment{ID: "ship_report", StudyID: study.ID, OriginSiteID: origin.ID, DestinationSiteID: destination.ID, ContainerID: container.ID, Reference: "REPORT-1", State: domain.ShipmentArrived, PlannedDispatchAt: now, ExpectedArrivalAt: now.Add(time.Hour), TotalVolumeMilliLit: batch.VolumeMilliLit, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := store.WithTx(ctx, func(tx repository.Tx) error {
		if err := tx.InsertShipment(ctx, shipment); err != nil {
			return err
		}
		batch.State = domain.SampleReceived
		batch.ShipmentID = shipment.ID
		if err := tx.UpdateSampleBatch(ctx, batch, batch.Version); err != nil {
			return err
		}
		return tx.InsertShipmentItem(ctx, domain.ShipmentItem{ShipmentID: shipment.ID, BatchID: batch.ID, AddedAt: now})
	}); err != nil {
		t.Fatal(err)
	}
	var report domain.ShipmentReconciliation
	if err := store.Read(ctx, func(reader repository.Reader) error {
		var err error
		report, err = reader.GetShipmentReconciliation(ctx, shipment.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if report.ExpectedBatchCount != 1 || report.ReceivedBatchCount != 1 || !report.Complete {
		t.Fatalf("report = %+v", report)
	}
}
