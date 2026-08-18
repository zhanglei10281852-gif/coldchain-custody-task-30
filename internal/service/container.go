package service

import (
	"context"
	"strings"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

type ContainerService struct{ dependencies }

func (s *ContainerService) StartCleaning(ctx context.Context, containerID string) (domain.Container, error) {
	return s.change(ctx, containerID, "container_cleaning_started", func(container *domain.Container) error {
		return container.StartCleaning(s.clock.Now())
	})
}

func (s *ContainerService) CompleteCleaning(ctx context.Context, containerID string) (domain.Container, error) {
	return s.change(ctx, containerID, "container_cleaning_completed", func(container *domain.Container) error {
		return container.CompleteCleaning(s.clock.Now())
	})
}

func (s *ContainerService) Retire(ctx context.Context, containerID, reason string) (domain.Container, error) {
	if strings.TrimSpace(reason) == "" {
		return domain.Container{}, domain.FieldError{Field: "reason", Message: "is required"}
	}
	return s.changeWithMetadata(ctx, containerID, "container_retired", map[string]string{"reason": strings.TrimSpace(reason)}, func(container *domain.Container) error {
		return container.Retire(s.clock.Now())
	})
}

func (s *ContainerService) change(ctx context.Context, containerID, action string, mutate func(*domain.Container) error) (domain.Container, error) {
	return s.changeWithMetadata(ctx, containerID, action, nil, mutate)
}

func (s *ContainerService) changeWithMetadata(ctx context.Context, containerID, action string, metadata map[string]string, mutate func(*domain.Container) error) (domain.Container, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return domain.Container{}, err
	}
	var result domain.Container
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		container, err := tx.GetContainer(ctx, containerID)
		if err != nil {
			return err
		}
		before := container.Version
		if err := mutate(&container); err != nil {
			return err
		}
		if err := tx.UpdateContainer(ctx, container, before); err != nil {
			return err
		}
		result = container
		return s.audit.Record(ctx, tx, action, "container", container.ID, "success", metadata)
	})
	return result, err
}
