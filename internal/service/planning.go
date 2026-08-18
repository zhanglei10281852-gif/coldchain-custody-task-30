package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/idempotency"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/identity"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

type PlanningService struct{ dependencies }

type PlanShipmentInput struct {
	StudyID           string
	OriginSiteID      string
	DestinationSiteID string
	ContainerID       string
	Reference         string
	BatchIDs          []string
	PlannedDispatchAt time.Time
	ExpectedArrivalAt time.Time
	IdempotencyKey    string
}

func (s *PlanningService) PlanShipment(ctx context.Context, input PlanShipmentInput) (domain.Shipment, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return domain.Shipment{}, err
	}
	if len(input.BatchIDs) == 0 {
		return domain.Shipment{}, domain.FieldError{Field: "batch_ids", Message: "at least one batch is required"}
	}
	if err := domain.ValidateIdempotencyKey(input.IdempotencyKey); err != nil {
		return domain.Shipment{}, err
	}
	hash, err := idempotency.Hash(input)
	if err != nil {
		return domain.Shipment{}, err
	}
	var shipment domain.Shipment
	err = s.store.WithTx(ctx, func(tx repository.Tx) error {
		if existing, err := tx.GetIdempotency(ctx, "plan-shipment", input.IdempotencyKey); err == nil {
			if existing.RequestHash != hash {
				return domain.ConflictError{Resource: "idempotency_key", Reason: "request payload differs"}
			}
			return json.Unmarshal(existing.ResponseBody, &shipment)
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		study, err := tx.GetStudy(ctx, input.StudyID)
		if err != nil {
			return err
		}
		if !study.CanAcceptShipments() {
			return domain.ConflictError{Resource: "study", Reason: "study is not active"}
		}
		origin, err := tx.GetSite(ctx, input.OriginSiteID)
		if err != nil {
			return err
		}
		destination, err := tx.GetSite(ctx, input.DestinationSiteID)
		if err != nil {
			return err
		}
		if origin.Status != domain.SiteActive || destination.Status != domain.SiteActive {
			return domain.ConflictError{Resource: "site", Reason: "origin and destination must be active"}
		}
		if err := domain.ValidateRoute(origin, destination); err != nil {
			return err
		}
		businessDay, err := origin.BusinessDay(input.PlannedDispatchAt)
		if err != nil {
			return err
		}
		count, err := tx.CountSiteShipmentsForBusinessDay(ctx, origin.ID, businessDay)
		if err != nil {
			return err
		}
		if count >= origin.DailyLimit {
			return domain.ConflictError{Resource: "site", Reason: "daily shipment limit reached"}
		}
		container, err := tx.GetContainer(ctx, input.ContainerID)
		if err != nil {
			return err
		}
		if err := container.EligibleFor(input.PlannedDispatchAt, 1); err != nil {
			return err
		}
		now := s.clock.Now()
		shipment = domain.Shipment{ID: identity.New("ship"), StudyID: input.StudyID, OriginSiteID: input.OriginSiteID,
			DestinationSiteID: input.DestinationSiteID, ContainerID: input.ContainerID, Reference: strings.TrimSpace(input.Reference),
			State: domain.ShipmentPlanned, PlannedDispatchAt: input.PlannedDispatchAt.UTC(), ExpectedArrivalAt: input.ExpectedArrivalAt.UTC(), Version: 1, CreatedAt: now, UpdatedAt: now}
		volume := 0
		batches := make([]domain.SampleBatch, 0, len(input.BatchIDs))
		seen := make(map[string]struct{}, len(input.BatchIDs))
		for _, batchID := range input.BatchIDs {
			if _, exists := seen[batchID]; exists {
				return domain.ConflictError{Resource: "sample_batch", Reason: "duplicate batch in request"}
			}
			seen[batchID] = struct{}{}
			batch, err := tx.GetSampleBatch(ctx, batchID)
			if err != nil {
				return err
			}
			if batch.StudyID != study.ID || batch.OriginSiteID != origin.ID {
				return domain.ConflictError{Resource: "sample_batch", Reason: "batch belongs to another study or site"}
			}
			if err := batch.Transition(domain.SampleReserved, now); err != nil {
				return err
			}
			volume += batch.VolumeMilliLit
			batches = append(batches, batch)
		}
		if err := (domain.TransitWindow{DispatchAt: input.PlannedDispatchAt.UTC(), ArrivalAt: input.ExpectedArrivalAt.UTC()}).Validate(study, batches, now); err != nil {
			return err
		}
		shipment.TotalVolumeMilliLit = volume
		if err := container.EligibleFor(input.PlannedDispatchAt, volume); err != nil {
			return err
		}
		if err := shipment.Validate(); err != nil {
			return err
		}
		if err := tx.InsertShipment(ctx, shipment); err != nil {
			return err
		}
		for _, batch := range batches {
			batch.ShipmentID = shipment.ID
			if err := tx.UpdateSampleBatch(ctx, batch, batch.Version); err != nil {
				return err
			}
			if err := tx.InsertShipmentItem(ctx, domain.ShipmentItem{ShipmentID: shipment.ID, BatchID: batch.ID, AddedAt: now}); err != nil {
				return err
			}
		}
		container.State = domain.ContainerReserved
		container.ReservedShipmentID = shipment.ID
		container.UpdatedAt = now
		if err := tx.UpdateContainer(ctx, container, container.Version); err != nil {
			return err
		}
		body, err := idempotency.Encode(shipment)
		if err != nil {
			return err
		}
		if err := tx.PutIdempotency(ctx, repository.IdempotencyRecord{Scope: "plan-shipment", Key: input.IdempotencyKey, RequestHash: hash, ResponseCode: 201, ResponseBody: body, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now}); err != nil {
			return err
		}
		if err := tx.InsertJob(ctx, domain.OutboxJob{ID: identity.New("job"), Kind: "shipment_planned", AggregateID: shipment.ID, Payload: body, Status: domain.JobPending, MaxAttempts: 5, AvailableAt: now, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, "shipment_planned", "shipment", shipment.ID, "success", map[string]string{"batch_count": fmt.Sprint(len(batches))})
	})
	return shipment, err
}

func (s *PlanningService) PackShipment(ctx context.Context, shipmentID string) (domain.Shipment, error) {
	return s.transition(ctx, shipmentID, domain.ShipmentPacked, domain.RoleOperations, "shipment_packed")
}

func (s *PlanningService) DispatchShipment(ctx context.Context, shipmentID string) (domain.Shipment, error) {
	return s.transitionAny(ctx, shipmentID, domain.ShipmentDispatched, []domain.Role{domain.RoleCourier, domain.RoleOperations}, "shipment_dispatched")
}

func (s *PlanningService) ArriveShipment(ctx context.Context, shipmentID string) (domain.Shipment, error) {
	return s.transitionAny(ctx, shipmentID, domain.ShipmentArrived, []domain.Role{domain.RoleCourier, domain.RoleOperations}, "shipment_arrived")
}

func (s *PlanningService) CloseShipment(ctx context.Context, shipmentID string) (domain.Shipment, error) {
	return s.transitionAny(ctx, shipmentID, domain.ShipmentClosed, []domain.Role{domain.RoleOperations}, "shipment_closed")
}

func (s *PlanningService) CancelShipment(ctx context.Context, shipmentID string, note string) (domain.Shipment, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return domain.Shipment{}, err
	}
	var result domain.Shipment
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		shipment, err := tx.GetShipment(ctx, shipmentID)
		if err != nil {
			return err
		}
		if shipment.State != domain.ShipmentPlanned && shipment.State != domain.ShipmentPacked {
			return domain.TransitionError{Entity: "shipment", From: string(shipment.State), To: string(domain.ShipmentCancelled)}
		}
		now := s.clock.Now()
		items, err := tx.ListShipmentItems(ctx, shipment.ID)
		if err != nil {
			return err
		}
		if err := shipment.Transition(domain.ShipmentCancelled, now); err != nil {
			return err
		}
		for _, batch := range items {
			if err := batch.Transition(domain.SampleReady, now); err != nil {
				return err
			}
			batch.ShipmentID = ""
			if err := tx.UpdateSampleBatch(ctx, batch, batch.Version); err != nil {
				return err
			}
		}
		container, err := tx.GetContainer(ctx, shipment.ContainerID)
		if err != nil {
			return err
		}
		container.State = domain.ContainerAvailable
		container.ReservedShipmentID = ""
		container.UpdatedAt = now
		if err := tx.UpdateContainer(ctx, container, container.Version); err != nil {
			return err
		}
		if err := tx.UpdateShipment(ctx, shipment, shipment.Version); err != nil {
			return err
		}
		result = shipment
		return s.audit.Record(ctx, tx, "shipment_cancelled", "shipment", shipment.ID, "success", map[string]string{"note": strings.TrimSpace(note)})
	})
	return result, err
}

func (s *PlanningService) transition(ctx context.Context, shipmentID string, target domain.ShipmentState, role domain.Role, action string) (domain.Shipment, error) {
	return s.transitionAny(ctx, shipmentID, target, []domain.Role{role}, action)
}

func (s *PlanningService) transitionAny(ctx context.Context, shipmentID string, target domain.ShipmentState, roles []domain.Role, action string) (domain.Shipment, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, roles...); err != nil {
		return domain.Shipment{}, err
	}
	var result domain.Shipment
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		shipment, err := tx.GetShipment(ctx, shipmentID)
		if err != nil {
			return err
		}
		if err := shipment.Transition(target, s.clock.Now()); err != nil {
			return err
		}
		now := s.clock.Now()
		items, err := tx.ListShipmentItems(ctx, shipment.ID)
		if err != nil {
			return err
		}
		for _, batch := range items {
			switch target {
			case domain.ShipmentDispatched:
				if err := batch.Transition(domain.SampleInTransit, now); err != nil {
					return err
				}
			case domain.ShipmentArrived:
				if batch.State != domain.SampleQuarantined && batch.State != domain.SampleDestroyed && batch.State != domain.SampleReleased {
					if err := batch.Transition(domain.SampleReceived, now); err != nil {
						return err
					}
				}
			case domain.ShipmentClosed:
				if batch.State != domain.SampleReleased && batch.State != domain.SampleDestroyed && batch.State != domain.SampleReceived {
					return domain.ConflictError{Resource: "sample_batch", Reason: "all samples must be resolved before closing"}
				}
			}
			if target == domain.ShipmentDispatched || target == domain.ShipmentArrived {
				if err := tx.UpdateSampleBatch(ctx, batch, batch.Version); err != nil {
					return err
				}
			}
		}
		container, err := tx.GetContainer(ctx, shipment.ContainerID)
		if err != nil {
			return err
		}
		switch target {
		case domain.ShipmentDispatched:
			container.State = domain.ContainerInTransit
		case domain.ShipmentClosed, domain.ShipmentCancelled:
			container.State = domain.ContainerAvailable
			container.ReservedShipmentID = ""
		}
		container.UpdatedAt = now
		if err := tx.UpdateContainer(ctx, container, container.Version); err != nil {
			return err
		}
		if err := tx.UpdateShipment(ctx, shipment, shipment.Version); err != nil {
			return err
		}
		result = shipment
		return s.audit.Record(ctx, tx, action, "shipment", shipment.ID, "success", nil)
	})
	return result, err
}
