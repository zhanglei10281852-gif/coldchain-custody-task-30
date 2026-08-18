package service

import (
	"context"
	"errors"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
)

type BulkSampleItem struct {
	Index int                 `json:"index"`
	Batch *domain.SampleBatch `json:"batch,omitempty"`
	Error string              `json:"error,omitempty"`
	Code  string              `json:"code"`
}

type BulkSampleResult struct {
	Items     []BulkSampleItem `json:"items"`
	Succeeded int              `json:"succeeded"`
	Failed    int              `json:"failed"`
}

func (s *CatalogService) BulkRegisterSamples(ctx context.Context, batches []domain.SampleBatch) (BulkSampleResult, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return BulkSampleResult{}, err
	}
	if len(batches) == 0 {
		return BulkSampleResult{}, domain.FieldError{Field: "batches", Message: "at least one batch is required"}
	}
	if len(batches) > 100 {
		return BulkSampleResult{}, domain.FieldError{Field: "batches", Message: "cannot contain more than 100 items"}
	}
	result := BulkSampleResult{Items: make([]BulkSampleItem, 0, len(batches))}
	for index, input := range batches {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		created, err := s.RegisterSample(ctx, input.Clone())
		if err != nil {
			result.Failed++
			result.Items = append(result.Items, BulkSampleItem{Index: index, Error: err.Error(), Code: classifyBulkError(err)})
			continue
		}
		result.Succeeded++
		createdCopy := created.Clone()
		result.Items = append(result.Items, BulkSampleItem{Index: index, Batch: &createdCopy, Code: "created"})
	}
	return result, nil
}

func classifyBulkError(err error) string {
	switch {
	case errors.Is(err, domain.ErrValidation):
		return "invalid"
	case errors.Is(err, domain.ErrConflict):
		return "conflict"
	default:
		return "failed"
	}
}
