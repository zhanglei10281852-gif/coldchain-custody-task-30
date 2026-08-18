package service

import (
	"context"
	"strings"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/identity"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

type CustodyService struct {
	dependencies
	handoffTTL time.Duration
}

type CreateHandoffInput struct {
	ShipmentID    string
	FromCustodian string
	ToCustodian   string
	Location      string
}

func (s *CustodyService) CreateHandoff(ctx context.Context, input CreateHandoffInput) (domain.CustodyHandoff, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleCourier, domain.RoleOperations); err != nil {
		return domain.CustodyHandoff{}, err
	}
	if strings.TrimSpace(input.FromCustodian) == "" {
		input.FromCustodian = principal.UserID
	}
	var result domain.CustodyHandoff
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		shipment, err := tx.GetShipment(ctx, input.ShipmentID)
		if err != nil {
			return err
		}
		if shipment.State != domain.ShipmentDispatched && shipment.State != domain.ShipmentArrived {
			return domain.ConflictError{Resource: "shipment", Reason: "handoff requires an active shipment"}
		}
		if _, err := tx.GetPendingHandoff(ctx, shipment.ID); err == nil {
			return domain.ConflictError{Resource: "handoff", Reason: "shipment already has a pending handoff"}
		} else if !isNotFound(err) {
			return err
		}
		now := s.clock.Now()
		handoff := domain.CustodyHandoff{ID: identity.New("handoff"), ShipmentID: shipment.ID, FromCustodian: strings.TrimSpace(input.FromCustodian), ToCustodian: strings.TrimSpace(input.ToCustodian), Location: strings.TrimSpace(input.Location), Status: domain.HandoffPending, ExpiresAt: now.Add(s.handoffTTL), Version: 1, CreatedAt: now, UpdatedAt: now}
		if err := handoff.Validate(); err != nil {
			return err
		}
		if err := tx.InsertHandoff(ctx, handoff); err != nil {
			return err
		}
		result = handoff
		return s.audit.Record(ctx, tx, "handoff_created", "custody_handoff", handoff.ID, "success", nil)
	})
	return result, err
}

func (s *CustodyService) ResolveHandoff(ctx context.Context, handoffID string, accepted bool, note string) (domain.CustodyHandoff, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleCourier, domain.RoleOperations); err != nil {
		return domain.CustodyHandoff{}, err
	}
	var result domain.CustodyHandoff
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		handoff, err := tx.GetHandoff(ctx, handoffID)
		if err != nil {
			return err
		}
		if handoff.ToCustodian != principal.UserID && !principal.Can(domain.RoleOperations) {
			return domain.ConflictError{Resource: "handoff", Reason: "only the receiving custodian may resolve it"}
		}
		status := domain.HandoffRejected
		if accepted {
			status = domain.HandoffAccepted
		}
		if err := handoff.Resolve(status, note, s.clock.Now()); err != nil {
			return err
		}
		if err := tx.UpdateHandoff(ctx, handoff, handoff.Version); err != nil {
			return err
		}
		result = handoff
		return s.audit.Record(ctx, tx, "handoff_resolved", "custody_handoff", handoff.ID, "success", map[string]string{"status": string(status)})
	})
	return result, err
}
