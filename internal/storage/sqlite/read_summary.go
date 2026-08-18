package sqlite

import (
	"context"
	"fmt"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
)

func (q *queries) GetOperationalSummary(ctx context.Context) (repository.OperationalSummary, error) {
	var summary repository.OperationalSummary
	queries := []struct {
		name   string
		target *int
		sql    string
	}{
		{"active studies", &summary.StudiesActive, `SELECT COUNT(*) FROM studies WHERE status = 'active'`},
		{"ready samples", &summary.SamplesReady, `SELECT COUNT(*) FROM sample_batches WHERE state = 'ready'`},
		{"transit samples", &summary.SamplesInTransit, `SELECT COUNT(*) FROM sample_batches WHERE state = 'in_transit'`},
		{"quarantined samples", &summary.SamplesQuarantined, `SELECT COUNT(*) FROM sample_batches WHERE state = 'quarantined'`},
		{"available containers", &summary.ContainersAvailable, `SELECT COUNT(*) FROM containers WHERE state = 'available'`},
		{"active shipments", &summary.ShipmentsActive, `SELECT COUNT(*) FROM shipments WHERE state IN ('planned', 'packed', 'dispatched', 'arrived')`},
		{"open excursions", &summary.OpenExcursions, `SELECT COUNT(*) FROM excursions WHERE status IN ('open', 'reviewing')`},
		{"pending handoffs", &summary.PendingHandoffs, `SELECT COUNT(*) FROM custody_handoffs WHERE status = 'pending'`},
		{"failed jobs", &summary.FailedJobs, `SELECT COUNT(*) FROM outbox_jobs WHERE status IN ('failed', 'dead')`},
	}
	for _, item := range queries {
		if err := q.q.QueryRowContext(ctx, item.sql).Scan(item.target); err != nil {
			return repository.OperationalSummary{}, fmt.Errorf("count %s: %w", item.name, err)
		}
	}
	return summary, nil
}
