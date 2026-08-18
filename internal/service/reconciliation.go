package service

import (
	"context"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

func (s *QueryService) ReconcileShipment(ctx context.Context, shipmentID string) (domain.ShipmentReconciliation, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations, domain.RoleCourier, domain.RoleReviewer, domain.RoleAuditor); err != nil {
		return domain.ShipmentReconciliation{}, err
	}
	var report domain.ShipmentReconciliation
	err := s.store.Read(ctx, func(reader repository.Reader) error {
		var err error
		report, err = reader.GetShipmentReconciliation(ctx, shipmentID)
		return err
	})
	return report, err
}
