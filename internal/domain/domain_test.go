package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestTemperatureRangeBoundaries(t *testing.T) {
	min := MilliCelsius(2000)
	max := MilliCelsius(8000)
	rangeValue, err := NewTemperatureRange(min, max)
	if err != nil {
		t.Fatalf("create range: %v", err)
	}
	tests := []struct {
		name  string
		value MilliCelsius
		want  bool
	}{
		{name: "minimum included", value: min, want: true},
		{name: "middle included", value: 5000, want: true},
		{name: "maximum included", value: max, want: true},
		{name: "below minimum", value: 1999, want: false},
		{name: "above maximum", value: 8001, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rangeValue.Contains(test.value); got != test.want {
				t.Fatalf("Contains(%d) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestTemperatureParsingRejectsInvalidValues(t *testing.T) {
	for _, value := range []float64{-197, 101, math.NaN()} {
		_, err := TemperatureFromCelsius(value)
		if err == nil {
			t.Fatalf("TemperatureFromCelsius(%v) succeeded", value)
		}
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("error %v does not wrap validation", err)
		}
	}
}

func TestSampleTransitionTable(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	base := SampleBatch{State: SampleRegistered, ExpiresAt: now.Add(24 * time.Hour)}
	cases := []struct {
		name string
		from SampleState
		to   SampleState
		want bool
	}{
		{"registered to ready", SampleRegistered, SampleReady, true},
		{"ready to reserved", SampleReady, SampleReserved, true},
		{"reserved to in transit", SampleReserved, SampleInTransit, true},
		{"in transit to received", SampleInTransit, SampleReceived, true},
		{"received to released", SampleReceived, SampleReleased, true},
		{"received to quarantine", SampleReceived, SampleQuarantined, true},
		{"quarantine to destroyed", SampleQuarantined, SampleDestroyed, true},
		{"registered to released", SampleRegistered, SampleReleased, false},
		{"released to ready", SampleReleased, SampleReady, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			batch := base
			batch.State = test.from
			err := batch.Transition(test.to, now)
			if (err == nil) != test.want {
				t.Fatalf("transition %s -> %s error = %v, want allowed=%v", test.from, test.to, err, test.want)
			}
			if test.want && batch.State != test.to {
				t.Fatalf("state = %s, want %s", batch.State, test.to)
			}
		})
	}
}

func TestExpiredSampleCanOnlyBeDestroyedOrQuarantined(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	batch := SampleBatch{State: SampleReceived, ExpiresAt: now.Add(-time.Minute)}
	if err := batch.Transition(SampleReleased, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("expired release error = %v, want conflict", err)
	}
	batch.State = SampleReceived
	if err := batch.Transition(SampleQuarantined, now); err != nil {
		t.Fatalf("expired quarantine failed: %v", err)
	}
}

func TestShipmentTransitionSetsTimestamps(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	shipment := Shipment{State: ShipmentPlanned, PlannedDispatchAt: now, ExpectedArrivalAt: now.Add(2 * time.Hour)}
	if err := shipment.Transition(ShipmentPacked, now); err != nil {
		t.Fatalf("pack: %v", err)
	}
	if err := shipment.Transition(ShipmentDispatched, now.Add(time.Minute)); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if shipment.DispatchedAt == nil || !shipment.DispatchedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("dispatched_at = %v", shipment.DispatchedAt)
	}
	if err := shipment.Transition(ShipmentArrived, now.Add(time.Hour)); err != nil {
		t.Fatalf("arrive: %v", err)
	}
	if err := shipment.Transition(ShipmentClosed, now.Add(2*time.Hour)); err != nil {
		t.Fatalf("close: %v", err)
	}
	if shipment.ClosedAt == nil {
		t.Fatal("closed_at is nil")
	}
}

func TestShipmentRejectsSkippedState(t *testing.T) {
	shipment := Shipment{State: ShipmentPlanned}
	err := shipment.Transition(ShipmentArrived, time.Now())
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("error = %v, want invalid transition", err)
	}
}

func TestHandoffResolutionAndExpiry(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	handoff := CustodyHandoff{Status: HandoffPending, ExpiresAt: now.Add(time.Hour)}
	if err := handoff.Resolve(HandoffAccepted, "seal intact", now); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if handoff.Status != HandoffAccepted || handoff.ResolvedAt == nil {
		t.Fatalf("handoff after accept = %+v", handoff)
	}
	handoff = CustodyHandoff{Status: HandoffPending, ExpiresAt: now.Add(-time.Minute)}
	if err := handoff.Resolve(HandoffAccepted, "", now); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired accept error = %v", err)
	}
	if err := handoff.Resolve(HandoffExpired, "expired", now); err != nil {
		t.Fatalf("expire: %v", err)
	}
}

func TestExcursionAggregatesReadings(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	excursion := Excursion{Status: ExcursionOpen}
	readings := []TemperatureReading{
		{Temperature: 9200, RecordedAt: now.Add(10 * time.Minute)},
		{Temperature: 8500, RecordedAt: now.Add(5 * time.Minute)},
		{Temperature: 11000, RecordedAt: now.Add(20 * time.Minute)},
	}
	for _, reading := range readings {
		excursion.Include(reading, now)
	}
	if excursion.ReadingCount != 3 || excursion.Minimum != 8500 || excursion.Maximum != 11000 {
		t.Fatalf("aggregate = %+v", excursion)
	}
	if !excursion.FirstReadingAt.Equal(now.Add(5*time.Minute)) || !excursion.LastReadingAt.Equal(now.Add(20*time.Minute)) {
		t.Fatalf("reading window = %v..%v", excursion.FirstReadingAt, excursion.LastReadingAt)
	}
}

func TestExcursionDecisionTable(t *testing.T) {
	now := time.Now().UTC()
	for _, decision := range []ExcursionStatus{ExcursionCleared, ExcursionRejected} {
		excursion := Excursion{Status: ExcursionReviewing}
		if err := excursion.Decide(decision, now); err != nil {
			t.Fatalf("decision %s: %v", decision, err)
		}
		if excursion.Status != decision {
			t.Fatalf("status = %s, want %s", excursion.Status, decision)
		}
	}
	excursion := Excursion{Status: ExcursionCleared}
	if err := excursion.Decide(ExcursionRejected, now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("decide closed excursion = %v", err)
	}
}

func TestSiteBusinessDayUsesCutoffAndTimezone(t *testing.T) {
	site := Site{Timezone: "Asia/Shanghai", CutoffHour: 6}
	before, err := site.BusinessDay(time.Date(2026, 8, 18, 21, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if before != "2026-08-18" {
		t.Fatalf("business day = %s", before)
	}
	after, err := site.BusinessDay(time.Date(2026, 8, 18, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if after != "2026-08-19" {
		t.Fatalf("business day after cutoff = %s", after)
	}
}

func TestContainerEligibility(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	base := Container{State: ContainerAvailable, CapacityMilliLit: 1000, CalibrationDueAt: now.Add(time.Hour)}
	if err := base.EligibleFor(now, 1000); err != nil {
		t.Fatalf("capacity boundary: %v", err)
	}
	if err := base.EligibleFor(now, 1001); !errors.Is(err, ErrCapacityExceeded) {
		t.Fatalf("capacity overflow = %v", err)
	}
	base.State = ContainerReserved
	if err := base.EligibleFor(now, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("reserved container = %v", err)
	}
}

func TestReconciliationEvaluate(t *testing.T) {
	report := ShipmentReconciliation{ShipmentState: ShipmentArrived, ExpectedBatchCount: 2, ReceivedBatchCount: 2, PendingHandoff: true}
	report.Evaluate()
	if report.Complete || len(report.Blockers) != 1 || report.Blockers[0] != "pending custody handoff" {
		t.Fatalf("report = %+v", report)
	}
	report.PendingHandoff = false
	report.Evaluate()
	if !report.Complete {
		t.Fatalf("resolved report = %+v", report)
	}
}

func TestAuditAndJobCloneIsolation(t *testing.T) {
	event := AuditEvent{Metadata: map[string]string{"one": "1"}}
	clone := event.Clone()
	clone.Metadata["one"] = "2"
	if event.Metadata["one"] != "1" {
		t.Fatal("audit metadata was shared")
	}
	job := OutboxJob{Payload: []byte("payload")}
	jobClone := job.Clone()
	jobClone.Payload[0] = 'P'
	if string(job.Payload) != "payload" {
		t.Fatal("job payload was shared")
	}
}

func TestTransitWindowChecksStudyLimitAndExpiry(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	study := Study{MaxTransit: 2 * time.Hour}
	batch := SampleBatch{ExpiresAt: now.Add(4 * time.Hour)}
	valid := TransitWindow{DispatchAt: now.Add(time.Hour), ArrivalAt: now.Add(2 * time.Hour)}
	if err := valid.Validate(study, []SampleBatch{batch}, now); err != nil {
		t.Fatalf("valid window: %v", err)
	}
	tooLong := TransitWindow{DispatchAt: now.Add(time.Hour), ArrivalAt: now.Add(4 * time.Hour)}
	if err := tooLong.Validate(study, []SampleBatch{batch}, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("long window = %v", err)
	}
	batch.ExpiresAt = now.Add(90 * time.Minute)
	if err := valid.Validate(study, []SampleBatch{batch}, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("expired batch window = %v", err)
	}
}

func TestPrincipalActionMatrix(t *testing.T) {
	cases := []struct {
		role   Role
		action Action
		want   bool
	}{
		{RoleOperations, ActionPlanShipment, true},
		{RoleOperations, ActionReviewExcursion, false},
		{RoleCourier, ActionRecordTelemetry, true},
		{RoleCourier, ActionCatalogWrite, false},
		{RoleReviewer, ActionReviewExcursion, true},
		{RoleAuditor, ActionReadAudit, true},
		{RoleAuditor, ActionMoveShipment, false},
	}
	for _, test := range cases {
		principal := Principal{Role: test.role}
		if got := principal.CanAction(test.action); got != test.want {
			t.Fatalf("%s %s = %v, want %v", test.role, test.action, got, test.want)
		}
	}
}

func TestIdentifierNormalizationAndValidation(t *testing.T) {
	if got := NormalizeCode("  site-sh-01 "); got != "SITE-SH-01" {
		t.Fatalf("normalized code = %q", got)
	}
	for _, value := range []string{"A", "with spaces", "ümlaut", "", strings.Repeat("X", 65)} {
		if err := ValidateBusinessCode("code", value); err == nil {
			t.Fatalf("invalid code %q passed", value)
		}
	}
	for _, value := range []string{"valid-key", "request-1234", strings.Repeat("x", 128)} {
		if err := ValidateIdempotencyKey(value); err != nil {
			t.Fatalf("valid idempotency key %q: %v", value, err)
		}
	}
	for _, value := range []string{"short", "line\nbreak", strings.Repeat("x", 129)} {
		if err := ValidateIdempotencyKey(value); err == nil {
			t.Fatalf("invalid idempotency key %q passed", value)
		}
	}
}

func TestTerminalStateHelpers(t *testing.T) {
	if !ShipmentClosed.IsTerminal() || !ShipmentCancelled.IsTerminal() || ShipmentArrived.IsTerminal() {
		t.Fatal("shipment terminal states are incorrect")
	}
	if !SampleReleased.IsTerminal() || !SampleDestroyed.IsTerminal() || SampleQuarantined.IsTerminal() {
		t.Fatal("sample terminal states are incorrect")
	}
	if !ExcursionCleared.IsResolved() || !ExcursionRejected.IsResolved() || ExcursionOpen.IsResolved() {
		t.Fatal("excursion resolved states are incorrect")
	}
}
