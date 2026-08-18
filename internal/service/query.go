package service

import (
	"context"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

type QueryService struct{ dependencies }

func (s *QueryService) OperationalSummary(ctx context.Context) (repository.OperationalSummary, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireAction(principal, domain.ActionReadOperations); err != nil {
		return repository.OperationalSummary{}, err
	}
	var summary repository.OperationalSummary
	err := s.store.Read(ctx, func(reader repository.Reader) error {
		var err error
		summary, err = reader.GetOperationalSummary(ctx)
		return err
	})
	return summary, err
}

func (s *QueryService) Shipment(ctx context.Context, id string) (domain.Shipment, []domain.SampleBatch, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations, domain.RoleCourier, domain.RoleReviewer, domain.RoleAuditor); err != nil {
		return domain.Shipment{}, nil, err
	}
	var shipment domain.Shipment
	var items []domain.SampleBatch
	err := s.store.Read(ctx, func(reader repository.Reader) error {
		var err error
		shipment, err = reader.GetShipment(ctx, id)
		if err != nil {
			return err
		}
		items, err = reader.ListShipmentItems(ctx, id)
		return err
	})
	return shipment, items, err
}

func (s *QueryService) Shipments(ctx context.Context, filter repository.ShipmentFilter) (repository.ShipmentPage, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations, domain.RoleCourier, domain.RoleReviewer, domain.RoleAuditor); err != nil {
		return repository.ShipmentPage{}, err
	}
	var page repository.ShipmentPage
	err := s.store.Read(ctx, func(reader repository.Reader) error {
		var err error
		page, err = reader.ListShipments(ctx, filter)
		return err
	})
	return page, err
}

func (s *QueryService) Samples(ctx context.Context, filter repository.SampleFilter) (repository.SamplePage, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations, domain.RoleCourier, domain.RoleReviewer, domain.RoleAuditor); err != nil {
		return repository.SamplePage{}, err
	}
	var page repository.SamplePage
	err := s.store.Read(ctx, func(reader repository.Reader) error {
		var err error
		page, err = reader.ListSamples(ctx, filter)
		return err
	})
	return page, err
}

func (s *QueryService) Excursions(ctx context.Context, filter repository.ExcursionFilter) (repository.ExcursionPage, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations, domain.RoleReviewer, domain.RoleAuditor); err != nil {
		return repository.ExcursionPage{}, err
	}
	var page repository.ExcursionPage
	err := s.store.Read(ctx, func(reader repository.Reader) error {
		var err error
		page, err = reader.ListExcursions(ctx, filter)
		return err
	})
	return page, err
}

func (s *QueryService) Audit(ctx context.Context, filter repository.AuditFilter) (repository.AuditPage, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleAuditor, domain.RoleOperations); err != nil {
		return repository.AuditPage{}, err
	}
	var page repository.AuditPage
	err := s.store.Read(ctx, func(reader repository.Reader) error {
		var err error
		page, err = reader.ListAuditEvents(ctx, filter)
		return err
	})
	return page, err
}
