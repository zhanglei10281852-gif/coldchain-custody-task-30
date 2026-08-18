package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/audit"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/clock"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

type Services struct {
	Auth       *AuthService
	Catalog    *CatalogService
	Containers *ContainerService
	Planning   *PlanningService
	Custody    *CustodyService
	Telemetry  *TelemetryService
	Review     *ReviewService
	Query      *QueryService
}

type dependencies struct {
	store repository.Store
	clock clock.Clock
	audit audit.Recorder
}

func New(store repository.Store, c clock.Clock, sessionTTL, handoffTTL time.Duration) *Services {
	deps := dependencies{store: store, clock: c, audit: audit.NewRecorder(c)}
	return &Services{
		Auth:       &AuthService{dependencies: deps, sessionTTL: sessionTTL},
		Catalog:    &CatalogService{dependencies: deps},
		Containers: &ContainerService{dependencies: deps},
		Planning:   &PlanningService{dependencies: deps},
		Custody:    &CustodyService{dependencies: deps, handoffTTL: handoffTTL},
		Telemetry:  &TelemetryService{dependencies: deps},
		Review:     &ReviewService{dependencies: deps},
		Query:      &QueryService{dependencies: deps},
	}
}

func requireRole(principal domain.Principal, roles ...domain.Role) error {
	if principal.UserID == "" {
		return fmt.Errorf("authentication required: %w", domain.ErrValidation)
	}
	if !principal.Can(roles...) {
		return fmt.Errorf("role %s is not permitted: %w", principal.Role, domain.ErrConflict)
	}
	return nil
}

func requireAction(principal domain.Principal, action domain.Action) error {
	if principal.UserID == "" {
		return fmt.Errorf("authentication required: %w", domain.ErrValidation)
	}
	if !principal.CanAction(action) {
		return fmt.Errorf("role %s cannot perform %s: %w", principal.Role, action, domain.ErrConflict)
	}
	return nil
}

func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
