package sqlite

import (
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

func TestFilteredShipmentTotalMatchesItems(t *testing.T) {
	store, ctx, now := testStore(t)
	study, origin, destination, container, _ := seedCatalog(t, store, ctx, now)
	shipments := []domain.Shipment{
		{ID: "ship_filter_1", StudyID: study.ID, OriginSiteID: origin.ID, DestinationSiteID: destination.ID, ContainerID: container.ID, Reference: "FILTER-1", State: domain.ShipmentPlanned, PlannedDispatchAt: now.Add(time.Hour), ExpectedArrivalAt: now.Add(2 * time.Hour), TotalVolumeMilliLit: 10, Version: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "ship_filter_2", StudyID: study.ID, OriginSiteID: origin.ID, DestinationSiteID: destination.ID, ContainerID: container.ID, Reference: "FILTER-2", State: domain.ShipmentPacked, PlannedDispatchAt: now.Add(2 * time.Hour), ExpectedArrivalAt: now.Add(3 * time.Hour), TotalVolumeMilliLit: 10, Version: 1, CreatedAt: now, UpdatedAt: now},
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
	if err := store.Read(ctx, func(reader repository.Reader) error {
		var err error
		page, err = reader.ListShipments(ctx, repository.ShipmentFilter{State: domain.ShipmentPlanned, Page: repository.PageRequest{Limit: 10}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != "ship_filter_1" {
		t.Fatalf("filtered page = %+v", page)
	}
}

func TestJobCannotBeClaimedTwice(t *testing.T) {
	store, ctx, now := testStore(t)
	job := domain.OutboxJob{ID: "job_once", Kind: "shipment_planned", AggregateID: "ship_once", Payload: []byte(`{}`), Status: domain.JobPending, MaxAttempts: 3, AvailableAt: now, CreatedAt: now, UpdatedAt: now}
	if err := store.WithTx(ctx, func(tx repository.Tx) error { return tx.InsertJob(ctx, job) }); err != nil {
		t.Fatal(err)
	}
	claim := func() []domain.OutboxJob {
		var jobs []domain.OutboxJob
		if err := store.WithTx(ctx, func(tx repository.Tx) error {
			var err error
			jobs, err = tx.ClaimJobs(ctx, now, 10)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return jobs
	}
	if jobs := claim(); len(jobs) != 1 {
		t.Fatalf("first claim = %+v", jobs)
	}
	if jobs := claim(); len(jobs) != 0 {
		t.Fatalf("second claim = %+v", jobs)
	}
}
