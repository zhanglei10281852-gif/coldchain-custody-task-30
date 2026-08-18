package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

func TestLoggedOutSessionCannotBeReused(t *testing.T) {
	f := newServiceFixture(t)
	login, err := f.services.Auth.Login(f.ctx, LoginInput{Email: f.operations.Email, Password: "very-secure-password"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.services.Auth.Logout(f.as(login.Principal), login.Principal); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Auth.Authenticate(f.ctx, login.Token); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("authentication after logout error = %v", err)
	}
}

func TestCancellationReleasesReservations(t *testing.T) {
	f := newServiceFixture(t)
	ctx := f.as(f.operations)
	shipment, err := f.services.Planning.PlanShipment(ctx, PlanShipmentInput{
		StudyID: f.study.ID, OriginSiteID: f.origin.ID, DestinationSiteID: f.destination.ID,
		ContainerID: f.container.ID, Reference: "SHIP-CANCEL", BatchIDs: []string{f.batch.ID},
		PlannedDispatchAt: f.clock.Now().Add(time.Hour), ExpectedArrivalAt: f.clock.Now().Add(2 * time.Hour), IdempotencyKey: "cancel-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.CancelShipment(ctx, shipment.ID, "route withdrawn"); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Read(ctx, func(reader repository.Reader) error {
		batch, err := reader.GetSampleBatch(ctx, f.batch.ID)
		if err != nil {
			return err
		}
		container, err := reader.GetContainer(ctx, f.container.ID)
		if err != nil {
			return err
		}
		if batch.State != domain.SampleReady || batch.ShipmentID != "" {
			t.Fatalf("cancelled batch = %+v", batch)
		}
		if container.State != domain.ContainerAvailable || container.ReservedShipmentID != "" {
			t.Fatalf("cancelled container = %+v", container)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestArrivalKeepsQuarantinedSamples(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-QUARANTINE")
	_, excursion, err := f.services.Telemetry.RecordReading(f.as(f.courier), RecordReadingInput{
		ShipmentID: shipment.ID, SensorID: "sensor-q", Sequence: 1, Temperature: 12000, RecordedAt: f.clock.Now().Add(time.Minute),
	})
	if err != nil || excursion == nil {
		t.Fatalf("record excursion = %+v, error = %v", excursion, err)
	}
	if _, err := f.services.Planning.ArriveShipment(f.as(f.courier), shipment.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Read(f.ctx, func(reader repository.Reader) error {
		batch, err := reader.GetSampleBatch(f.ctx, f.batch.ID)
		if err == nil && batch.State != domain.SampleQuarantined {
			t.Fatalf("arrived quarantined batch = %+v", batch)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestClosingWaitsForQualityResolution(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-BLOCKED-CLOSE")
	if _, _, err := f.services.Telemetry.RecordReading(f.as(f.courier), RecordReadingInput{
		ShipmentID: shipment.ID, SensorID: "sensor-close", Sequence: 1, Temperature: 12000, RecordedAt: f.clock.Now().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.ArriveShipment(f.as(f.courier), shipment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.CloseShipment(f.as(f.operations), shipment.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("close with unresolved samples error = %v", err)
	}
}

func TestSecondPendingHandoffIsRejected(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-HANDOFF-UNIQUE")
	input := CreateHandoffInput{ShipmentID: shipment.ID, FromCustodian: f.operations.UserID, ToCustodian: f.courier.UserID, Location: "Dock 4"}
	if _, err := f.services.Custody.CreateHandoff(f.as(f.courier), input); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Custody.CreateHandoff(f.as(f.courier), input); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second pending handoff error = %v", err)
	}
}

func TestExpiredHandoffCannotBeAccepted(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-HANDOFF-EXPIRED")
	handoff, err := f.services.Custody.CreateHandoff(f.as(f.courier), CreateHandoffInput{
		ShipmentID: shipment.ID, FromCustodian: f.operations.UserID, ToCustodian: f.courier.UserID, Location: "Dock 5",
	})
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(31 * time.Minute)
	if _, err := f.services.Custody.ResolveHandoff(f.as(f.courier), handoff.ID, true, "late acceptance"); !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("expired handoff error = %v", err)
	}
}

func TestExcursionAccumulatesReadings(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-AGGREGATE")
	for sequence, temperature := range []domain.MilliCelsius{12000, -1000} {
		_, _, err := f.services.Telemetry.RecordReading(f.as(f.courier), RecordReadingInput{
			ShipmentID: shipment.ID, SensorID: "sensor-a", Sequence: int64(sequence + 1), Temperature: temperature,
			RecordedAt: f.clock.Now().Add(time.Duration(sequence+1) * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.services.Query.Excursions(f.as(f.reviewer), repository.ExcursionFilter{ShipmentID: shipment.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ReadingCount != 2 || page.Items[0].Minimum != -1000 || page.Items[0].Maximum != 12000 {
		t.Fatalf("excursions = %+v", page)
	}
}

func TestRejectedReviewDestroysSamples(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-REJECT")
	_, excursion, err := f.services.Telemetry.RecordReading(f.as(f.courier), RecordReadingInput{
		ShipmentID: shipment.ID, SensorID: "sensor-r", Sequence: 1, Temperature: 12000, RecordedAt: f.clock.Now().Add(time.Minute),
	})
	if err != nil || excursion == nil {
		t.Fatalf("record excursion = %+v, error = %v", excursion, err)
	}
	if _, err := f.services.Review.Decide(f.as(f.reviewer), DecideInput{ExcursionID: excursion.ID, Decision: domain.ExcursionRejected, Rationale: "exposure exceeded protocol"}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Read(f.ctx, func(reader repository.Reader) error {
		batch, err := reader.GetSampleBatch(f.ctx, f.batch.ID)
		if err == nil && batch.State != domain.SampleDestroyed {
			t.Fatalf("rejected batch = %+v", batch)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReconciliationShowsPendingHandoff(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-RECONCILE-HANDOFF")
	if _, err := f.services.Custody.CreateHandoff(f.as(f.courier), CreateHandoffInput{
		ShipmentID: shipment.ID, FromCustodian: f.operations.UserID, ToCustodian: f.courier.UserID, Location: "Dock 6",
	}); err != nil {
		t.Fatal(err)
	}
	report, err := f.services.Query.ReconcileShipment(f.as(f.operations), shipment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !report.PendingHandoff || report.Complete {
		t.Fatalf("reconciliation = %+v", report)
	}
}

func TestBulkResultsOwnIndependentBatches(t *testing.T) {
	f := newServiceFixture(t)
	now := f.clock.Now()
	result, err := f.services.Catalog.BulkRegisterSamples(f.as(f.operations), []domain.SampleBatch{
		{StudyID: f.study.ID, OriginSiteID: f.origin.ID, ExternalRef: "BULK-A", SpecimenType: "serum", VialCount: 1, VolumeMilliLit: 20, ExpiresAt: now.Add(time.Hour)},
		{StudyID: f.study.ID, OriginSiteID: f.origin.ID, ExternalRef: "BULK-B", SpecimenType: "plasma", VialCount: 2, VolumeMilliLit: 30, ExpiresAt: now.Add(2 * time.Hour)},
	})
	if err != nil || result.Succeeded != 2 {
		t.Fatalf("bulk result = %+v, error = %v", result, err)
	}
	firstID := result.Items[0].Batch.ID
	result.Items[1].Batch.ID = "changed"
	if result.Items[0].Batch.ID != firstID || result.Items[0].Batch == result.Items[1].Batch {
		t.Fatalf("bulk item ownership = %+v", result.Items)
	}
}

func TestDuplicateReadingPreservesConflict(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-DUPLICATE-READING")
	input := RecordReadingInput{ShipmentID: shipment.ID, SensorID: "sensor-d", Sequence: 1, Temperature: 5000, RecordedAt: f.clock.Now()}
	if _, _, err := f.services.Telemetry.RecordReading(f.as(f.courier), input); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.services.Telemetry.RecordReading(f.as(f.courier), input); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate reading error = %v", err)
	}
}

func TestCourierCannotReadAuditTrail(t *testing.T) {
	f := newServiceFixture(t)
	if _, err := f.services.Query.Audit(f.as(f.courier), repository.AuditFilter{Page: repository.PageRequest{Limit: 10}}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("courier audit error = %v", err)
	}
}

func TestCancelledRegistrationDoesNotAdvance(t *testing.T) {
	f := newServiceFixture(t)
	registered, err := f.services.Catalog.RegisterSample(f.as(f.operations), domain.SampleBatch{
		StudyID: f.study.ID, OriginSiteID: f.origin.ID, ExternalRef: "CANCELLED-READY",
		SpecimenType: "serum", VialCount: 1, VolumeMilliLit: 10, ExpiresAt: f.clock.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(f.as(f.operations))
	cancel()
	if _, err := f.services.Catalog.MarkSampleReady(cancelled, registered.ID); err == nil {
		t.Fatal("cancelled state change succeeded")
	}
	if err := f.store.Read(f.ctx, func(reader repository.Reader) error {
		batch, err := reader.GetSampleBatch(f.ctx, registered.ID)
		if err == nil && batch.State != domain.SampleRegistered {
			t.Fatalf("cancelled batch = %+v", batch)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}
