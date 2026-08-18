package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/clock"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/requestmeta"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/storage/sqlite"
)

type serviceFixture struct {
	t           *testing.T
	ctx         context.Context
	store       *sqlite.Store
	services    *Services
	clock       *clock.Fixed
	operations  domain.Principal
	courier     domain.Principal
	reviewer    domain.Principal
	study       domain.Study
	origin      domain.Site
	destination domain.Site
	container   domain.Container
	batch       domain.SampleBatch
}

func newServiceFixture(t *testing.T) *serviceFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	fixed := clock.NewFixed(now)
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "service.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	services := New(store, fixed, 4*time.Hour, 30*time.Minute)
	users := []struct {
		email string
		name  string
		role  domain.Role
	}{
		{"ops@example.test", "Ops", domain.RoleOperations},
		{"courier@example.test", "Courier", domain.RoleCourier},
		{"reviewer@example.test", "Reviewer", domain.RoleReviewer},
	}
	principals := make([]domain.Principal, 0, len(users))
	for _, user := range users {
		created, err := services.Auth.CreateUser(ctx, user.email, user.name, "very-secure-password", user.role)
		if err != nil {
			t.Fatalf("create user %s: %v", user.email, err)
		}
		login, err := services.Auth.Login(ctx, LoginInput{Email: user.email, Password: "very-secure-password"})
		if err != nil {
			t.Fatalf("login %s: %v", user.email, err)
		}
		if login.Principal.UserID != created.ID {
			t.Fatalf("principal user = %s, created = %s", login.Principal.UserID, created.ID)
		}
		principals = append(principals, login.Principal)
	}
	minimum, _ := domain.TemperatureFromCelsius(2)
	maximum, _ := domain.TemperatureFromCelsius(8)
	rangeValue, _ := domain.NewTemperatureRange(minimum, maximum)
	opsCtx := requestmeta.WithPrincipal(ctx, principals[0])
	study, err := services.Catalog.CreateStudy(opsCtx, domain.Study{Code: "STUDY-1", Name: "Cold study", Temperature: rangeValue, MaxTransit: 24 * time.Hour, ReviewDeadline: 4 * time.Hour, BusinessTimezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatal(err)
	}
	study, err = services.Catalog.ActivateStudy(opsCtx, study.ID)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := services.Catalog.CreateSite(opsCtx, domain.Site{Code: "SITE-1", Name: "Origin", Timezone: "Asia/Shanghai", DailyLimit: 10, CutoffHour: 6})
	if err != nil {
		t.Fatal(err)
	}
	destination, err := services.Catalog.CreateSite(opsCtx, domain.Site{Code: "SITE-2", Name: "Destination", Timezone: "Asia/Shanghai", DailyLimit: 10, CutoffHour: 6})
	if err != nil {
		t.Fatal(err)
	}
	now = fixed.Now()
	container, err := services.Catalog.CreateContainer(opsCtx, domain.Container{SerialNumber: "BOX-1", CapacityMilliLit: 1000, CalibrationDueAt: now.Add(48 * time.Hour), LastCleanedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := services.Catalog.RegisterSample(opsCtx, domain.SampleBatch{StudyID: study.ID, OriginSiteID: origin.ID, ExternalRef: "EXT-1", SpecimenType: "plasma", VialCount: 2, VolumeMilliLit: 100, ExpiresAt: now.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	batch, err = services.Catalog.MarkSampleReady(opsCtx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	return &serviceFixture{t: t, ctx: ctx, store: store, services: services, clock: fixed, operations: principals[0], courier: principals[1], reviewer: principals[2], study: study, origin: origin, destination: destination, container: container, batch: batch}
}

func (f *serviceFixture) as(principal domain.Principal) context.Context {
	return requestmeta.WithPrincipal(requestmeta.WithRequestID(f.ctx, "req-test"), principal)
}

func TestAuthRejectsWrongPasswordAndHonorsLogout(t *testing.T) {
	f := newServiceFixture(t)
	if _, err := f.services.Auth.Login(f.ctx, LoginInput{Email: f.operations.Email, Password: "wrong-password"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("wrong password error = %v", err)
	}
	if err := f.services.Auth.Logout(f.as(f.operations), f.operations); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Auth.Authenticate(f.ctx, "missing-token"); err == nil {
		t.Fatal("missing token authenticated")
	}
}

func TestPlanningIsIdempotentAndReservesRelatedEntities(t *testing.T) {
	f := newServiceFixture(t)
	input := PlanShipmentInput{StudyID: f.study.ID, OriginSiteID: f.origin.ID, DestinationSiteID: f.destination.ID, ContainerID: f.container.ID, Reference: "SHIP-1", BatchIDs: []string{f.batch.ID}, PlannedDispatchAt: f.clock.Now().Add(time.Hour), ExpectedArrivalAt: f.clock.Now().Add(2 * time.Hour), IdempotencyKey: "plan-key"}
	ctx := f.as(f.operations)
	first, err := f.services.Planning.PlanShipment(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.services.Planning.PlanShipment(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.Reference != "SHIP-1" {
		t.Fatalf("idempotent responses differ: %+v / %+v", first, second)
	}
	if err := f.store.Read(ctx, func(reader repository.Reader) error {
		batch, err := reader.GetSampleBatch(ctx, f.batch.ID)
		if err != nil {
			return err
		}
		if batch.State != domain.SampleReserved || batch.ShipmentID != first.ID {
			t.Fatalf("reserved batch = %+v", batch)
		}
		container, err := reader.GetContainer(ctx, f.container.ID)
		if err != nil {
			return err
		}
		if container.State != domain.ContainerReserved || container.ReservedShipmentID != first.ID {
			t.Fatalf("reserved container = %+v", container)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPlanningRejectsDifferentIdempotencyPayload(t *testing.T) {
	f := newServiceFixture(t)
	input := PlanShipmentInput{StudyID: f.study.ID, OriginSiteID: f.origin.ID, DestinationSiteID: f.destination.ID, ContainerID: f.container.ID, Reference: "SHIP-1", BatchIDs: []string{f.batch.ID}, PlannedDispatchAt: f.clock.Now().Add(time.Hour), ExpectedArrivalAt: f.clock.Now().Add(2 * time.Hour), IdempotencyKey: "plan-key"}
	ctx := f.as(f.operations)
	if _, err := f.services.Planning.PlanShipment(ctx, input); err != nil {
		t.Fatal(err)
	}
	input.Reference = "SHIP-OTHER"
	if _, err := f.services.Planning.PlanShipment(ctx, input); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("different payload error = %v", err)
	}
}

func TestPlanningLifecycleMovesSamplesAndContainer(t *testing.T) {
	f := newServiceFixture(t)
	ctx := f.as(f.operations)
	shipment, err := f.services.Planning.PlanShipment(ctx, PlanShipmentInput{StudyID: f.study.ID, OriginSiteID: f.origin.ID, DestinationSiteID: f.destination.ID, ContainerID: f.container.ID, Reference: "SHIP-LIFE", BatchIDs: []string{f.batch.ID}, PlannedDispatchAt: f.clock.Now().Add(time.Hour), ExpectedArrivalAt: f.clock.Now().Add(2 * time.Hour), IdempotencyKey: "life-key"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.PackShipment(ctx, shipment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.DispatchShipment(f.as(f.courier), shipment.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Read(ctx, func(reader repository.Reader) error {
		batch, err := reader.GetSampleBatch(ctx, f.batch.ID)
		if err != nil {
			return err
		}
		if batch.State != domain.SampleInTransit {
			t.Fatalf("in transit batch = %+v", batch)
		}
		container, err := reader.GetContainer(ctx, f.container.ID)
		if err != nil {
			return err
		}
		if container.State != domain.ContainerInTransit {
			t.Fatalf("in transit container = %+v", container)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.ArriveShipment(f.as(f.courier), shipment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.CloseShipment(ctx, shipment.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCustodyOnlyReceiverCanResolve(t *testing.T) {
	f := newServiceFixture(t)
	ctx := f.as(f.operations)
	shipment, err := f.services.Planning.PlanShipment(ctx, PlanShipmentInput{StudyID: f.study.ID, OriginSiteID: f.origin.ID, DestinationSiteID: f.destination.ID, ContainerID: f.container.ID, Reference: "SHIP-HAND", BatchIDs: []string{f.batch.ID}, PlannedDispatchAt: f.clock.Now().Add(time.Hour), ExpectedArrivalAt: f.clock.Now().Add(2 * time.Hour), IdempotencyKey: "hand-key"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.PackShipment(ctx, shipment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.DispatchShipment(f.as(f.courier), shipment.ID); err != nil {
		t.Fatal(err)
	}
	handoff, err := f.services.Custody.CreateHandoff(f.as(f.courier), CreateHandoffInput{ShipmentID: shipment.ID, FromCustodian: f.operations.UserID, ToCustodian: f.courier.UserID, Location: "Dock 2"})
	if err != nil {
		t.Fatal(err)
	}
	other := domain.Principal{UserID: "auditor-user", Role: domain.RoleAuditor}
	if _, err := f.services.Custody.ResolveHandoff(f.as(other), handoff.ID, true, "wrong actor"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("wrong actor error = %v", err)
	}
	if _, err := f.services.Custody.ResolveHandoff(f.as(f.courier), handoff.ID, true, "seal intact"); err != nil {
		t.Fatal(err)
	}
}

func (f *serviceFixture) planAndDispatch(t *testing.T, ref string) domain.Shipment {
	t.Helper()
	shipment, err := f.services.Planning.PlanShipment(f.as(f.operations), PlanShipmentInput{StudyID: f.study.ID, OriginSiteID: f.origin.ID, DestinationSiteID: f.destination.ID, ContainerID: f.container.ID, Reference: ref, BatchIDs: []string{f.batch.ID}, PlannedDispatchAt: f.clock.Now().Add(time.Hour), ExpectedArrivalAt: f.clock.Now().Add(2 * time.Hour), IdempotencyKey: ref + "-key"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.PackShipment(f.as(f.operations), shipment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Planning.DispatchShipment(f.as(f.courier), shipment.ID); err != nil {
		t.Fatal(err)
	}
	return shipment
}

func TestTemperatureExcursionQuarantinesAndReviewerClears(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-TEMP")
	reading, excursion, err := f.services.Telemetry.RecordReading(f.as(f.courier), RecordReadingInput{ShipmentID: shipment.ID, SensorID: "sensor-1", Sequence: 1, Temperature: 12000, RecordedAt: f.clock.Now().Add(10 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if reading.ID == "" || excursion == nil || excursion.ReadingCount != 1 {
		t.Fatalf("reading=%+v excursion=%+v", reading, excursion)
	}
	if _, err := f.services.Planning.ArriveShipment(f.as(f.courier), shipment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Review.StartReview(f.as(f.reviewer), excursion.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.services.Review.Decide(f.as(f.reviewer), DecideInput{ExcursionID: excursion.ID, Decision: domain.ExcursionCleared, Rationale: "validated logger trace"}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Read(f.ctx, func(reader repository.Reader) error {
		batch, err := reader.GetSampleBatch(f.ctx, f.batch.ID)
		if err != nil {
			return err
		}
		if batch.State != domain.SampleReleased {
			t.Fatalf("batch after clear = %+v", batch)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestInRangeReadingDoesNotOpenExcursion(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-IN-RANGE")
	_, excursion, err := f.services.Telemetry.RecordReading(f.as(f.courier), RecordReadingInput{ShipmentID: shipment.ID, SensorID: "sensor-1", Sequence: 1, Temperature: 5000, RecordedAt: f.clock.Now()})
	if err != nil || excursion != nil {
		t.Fatalf("in range result excursion=%+v error=%v", excursion, err)
	}
}

func TestQueryReconciliationReportsBlockers(t *testing.T) {
	f := newServiceFixture(t)
	shipment := f.planAndDispatch(t, "SHIP-REPORT")
	report, err := f.services.Query.ReconcileShipment(f.as(f.operations), shipment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.ExpectedBatchCount != 1 || report.Complete {
		t.Fatalf("report = %+v", report)
	}
}

func TestContextCancellationReachesTransaction(t *testing.T) {
	f := newServiceFixture(t)
	cancelled, cancel := context.WithCancel(f.as(f.operations))
	cancel()
	_, err := f.services.Catalog.MarkSampleReady(cancelled, f.batch.ID)
	if err == nil {
		t.Fatal("cancelled command succeeded")
	}
}

func TestContainerCleaningAndRetirementLifecycle(t *testing.T) {
	f := newServiceFixture(t)
	ctx := f.as(f.operations)
	cleaning, err := f.services.Containers.StartCleaning(ctx, f.container.ID)
	if err != nil || cleaning.State != domain.ContainerCleaning {
		t.Fatalf("start cleaning = %+v, error=%v", cleaning, err)
	}
	f.clock.Advance(time.Hour)
	available, err := f.services.Containers.CompleteCleaning(ctx, f.container.ID)
	if err != nil || available.State != domain.ContainerAvailable || !available.LastCleanedAt.Equal(f.clock.Now()) {
		t.Fatalf("complete cleaning = %+v, error=%v", available, err)
	}
	retired, err := f.services.Containers.Retire(ctx, f.container.ID, "calibration program ended")
	if err != nil || retired.State != domain.ContainerRetired {
		t.Fatalf("retire = %+v, error=%v", retired, err)
	}
	if _, err := f.services.Containers.StartCleaning(ctx, f.container.ID); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("clean retired error = %v", err)
	}
}

func TestBulkRegistrationReturnsPartialFailures(t *testing.T) {
	f := newServiceFixture(t)
	now := f.clock.Now()
	inputs := []domain.SampleBatch{
		{StudyID: f.study.ID, OriginSiteID: f.origin.ID, ExternalRef: "BULK-OK", SpecimenType: "serum", VialCount: 1, VolumeMilliLit: 20, ExpiresAt: now.Add(time.Hour)},
		{StudyID: f.study.ID, OriginSiteID: f.origin.ID, ExternalRef: "", SpecimenType: "serum", VialCount: 1, VolumeMilliLit: 20, ExpiresAt: now.Add(time.Hour)},
		{StudyID: f.study.ID, OriginSiteID: f.origin.ID, ExternalRef: "BULK-OK", SpecimenType: "serum", VialCount: 1, VolumeMilliLit: 20, ExpiresAt: now.Add(time.Hour)},
	}
	result, err := f.services.Catalog.BulkRegisterSamples(f.as(f.operations), inputs)
	if err != nil {
		t.Fatal(err)
	}
	if result.Succeeded != 1 || result.Failed != 2 || len(result.Items) != 3 {
		t.Fatalf("bulk result = %+v", result)
	}
	if result.Items[0].Code != "created" || result.Items[1].Code != "invalid" || result.Items[2].Code != "conflict" {
		t.Fatalf("bulk item codes = %+v", result.Items)
	}
}

func TestOperationalSummaryRequiresReadPermissionAndCountsRows(t *testing.T) {
	f := newServiceFixture(t)
	if _, err := f.services.Query.OperationalSummary(f.as(f.reviewer)); err != nil {
		t.Fatalf("reviewer summary: %v", err)
	}
	summary, err := f.services.Query.OperationalSummary(f.as(f.operations))
	if err != nil {
		t.Fatal(err)
	}
	if summary.StudiesActive != 1 || summary.SamplesReady != 1 || summary.ContainersAvailable != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := f.services.Query.OperationalSummary(f.as(domain.Principal{UserID: "courier", Role: domain.RoleCourier})); err != nil {
		t.Fatalf("courier read summary: %v", err)
	}
}
