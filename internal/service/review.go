package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/identity"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

type ReviewService struct{ dependencies }

func (s *ReviewService) StartReview(ctx context.Context, excursionID string) (domain.Excursion, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleReviewer); err != nil {
		return domain.Excursion{}, err
	}
	var result domain.Excursion
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		excursion, err := tx.GetExcursion(ctx, excursionID)
		if err != nil {
			return err
		}
		before := excursion.Version
		if err := excursion.StartReview(s.clock.Now()); err != nil {
			return err
		}
		if err := tx.UpdateExcursion(ctx, excursion, before); err != nil {
			return err
		}
		result = excursion
		return s.audit.Record(ctx, tx, "excursion_review_started", "excursion", excursion.ID, "success", nil)
	})
	return result, err
}

type DecideInput struct {
	ExcursionID string
	Decision    domain.ExcursionStatus
	Rationale   string
}

func (s *ReviewService) Decide(ctx context.Context, input DecideInput) (domain.Excursion, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleReviewer); err != nil {
		return domain.Excursion{}, err
	}
	if strings.TrimSpace(input.Rationale) == "" {
		return domain.Excursion{}, domain.FieldError{Field: "rationale", Message: "is required"}
	}
	var result domain.Excursion
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		excursion, err := tx.GetExcursion(ctx, input.ExcursionID)
		if err != nil {
			return err
		}
		before := excursion.Version
		if err := excursion.Decide(input.Decision, s.clock.Now()); err != nil {
			return err
		}
		if err := tx.UpdateExcursion(ctx, excursion, before); err != nil {
			return err
		}
		shipment, err := tx.GetShipment(ctx, excursion.ShipmentID)
		if err != nil {
			return err
		}
		items, err := tx.ListShipmentItems(ctx, shipment.ID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		for _, batch := range items {
			switch input.Decision {
			case domain.ExcursionCleared:
				if batch.State != domain.SampleQuarantined {
					continue
				}
				batch.State = domain.SampleReleased
				batch.QuarantineNote = ""
			case domain.ExcursionRejected:
				if batch.State != domain.SampleQuarantined {
					continue
				}
				batch.State = domain.SampleDestroyed
				batch.QuarantineNote = strings.TrimSpace(input.Rationale)
			default:
				return fmt.Errorf("unsupported review decision: %w", domain.ErrValidation)
			}
			batch.UpdatedAt = now
			if err := tx.UpdateSampleBatch(ctx, batch, batch.Version); err != nil {
				return err
			}
		}
		decision := domain.ReviewDecision{ID: identity.New("decision"), ExcursionID: excursion.ID, Reviewer: principal.UserID, Decision: input.Decision, Rationale: strings.TrimSpace(input.Rationale), CreatedAt: now}
		if err := tx.InsertReviewDecision(ctx, decision); err != nil {
			return err
		}
		result = excursion
		return s.audit.Record(ctx, tx, "excursion_decided", "excursion", excursion.ID, "success", map[string]string{"decision": string(input.Decision)})
	})
	return result, err
}

func (s *ReviewService) EnsureReviewable(ctx context.Context, excursionID string) error {
	return s.store.Read(ctx, func(reader repository.Reader) error {
		excursion, err := reader.GetExcursion(ctx, excursionID)
		if err != nil {
			return err
		}
		if excursion.Status != domain.ExcursionOpen && excursion.Status != domain.ExcursionReviewing {
			return errors.New("excursion is already decided")
		}
		return nil
	})
}
