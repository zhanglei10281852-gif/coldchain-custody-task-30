package repository

import (
	"context"
	"time"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
)

type Store interface {
	WithTx(ctx context.Context, fn func(Tx) error) error
	Read(ctx context.Context, fn func(Reader) error) error
	Ping(ctx context.Context) error
	Close() error
}

type Reader interface {
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetUser(ctx context.Context, id string) (domain.User, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error)
	GetStudy(ctx context.Context, id string) (domain.Study, error)
	GetSite(ctx context.Context, id string) (domain.Site, error)
	GetSampleBatch(ctx context.Context, id string) (domain.SampleBatch, error)
	GetContainer(ctx context.Context, id string) (domain.Container, error)
	GetShipment(ctx context.Context, id string) (domain.Shipment, error)
	ListShipmentItems(ctx context.Context, shipmentID string) ([]domain.SampleBatch, error)
	GetPendingHandoff(ctx context.Context, shipmentID string) (domain.CustodyHandoff, error)
	GetHandoff(ctx context.Context, id string) (domain.CustodyHandoff, error)
	GetActiveExcursion(ctx context.Context, shipmentID string) (domain.Excursion, error)
	GetExcursion(ctx context.Context, id string) (domain.Excursion, error)
	GetShipmentReconciliation(ctx context.Context, shipmentID string) (domain.ShipmentReconciliation, error)
	GetOperationalSummary(ctx context.Context) (OperationalSummary, error)
	ListShipments(ctx context.Context, filter ShipmentFilter) (ShipmentPage, error)
	ListSamples(ctx context.Context, filter SampleFilter) (SamplePage, error)
	ListExcursions(ctx context.Context, filter ExcursionFilter) (ExcursionPage, error)
	ListAuditEvents(ctx context.Context, filter AuditFilter) (AuditPage, error)
	GetIdempotency(ctx context.Context, scope, key string) (IdempotencyRecord, error)
	CountSiteShipmentsForBusinessDay(ctx context.Context, siteID, businessDay string) (int, error)
}

type Tx interface {
	Reader
	InsertUser(ctx context.Context, user domain.User) error
	InsertSession(ctx context.Context, session domain.Session) error
	RevokeSession(ctx context.Context, sessionID string, revokedAt time.Time) error
	InsertStudy(ctx context.Context, study domain.Study) error
	UpdateStudy(ctx context.Context, study domain.Study, expectedVersion int64) error
	InsertSite(ctx context.Context, site domain.Site) error
	InsertSampleBatch(ctx context.Context, batch domain.SampleBatch) error
	UpdateSampleBatch(ctx context.Context, batch domain.SampleBatch, expectedVersion int64) error
	InsertContainer(ctx context.Context, container domain.Container) error
	UpdateContainer(ctx context.Context, container domain.Container, expectedVersion int64) error
	InsertShipment(ctx context.Context, shipment domain.Shipment) error
	UpdateShipment(ctx context.Context, shipment domain.Shipment, expectedVersion int64) error
	InsertShipmentItem(ctx context.Context, item domain.ShipmentItem) error
	InsertHandoff(ctx context.Context, handoff domain.CustodyHandoff) error
	UpdateHandoff(ctx context.Context, handoff domain.CustodyHandoff, expectedVersion int64) error
	InsertReading(ctx context.Context, reading domain.TemperatureReading) error
	InsertExcursion(ctx context.Context, excursion domain.Excursion) error
	UpdateExcursion(ctx context.Context, excursion domain.Excursion, expectedVersion int64) error
	InsertReviewDecision(ctx context.Context, decision domain.ReviewDecision) error
	InsertAuditEvent(ctx context.Context, event domain.AuditEvent) error
	PutIdempotency(ctx context.Context, record IdempotencyRecord) error
	InsertJob(ctx context.Context, job domain.OutboxJob) error
	ClaimJobs(ctx context.Context, now time.Time, limit int) ([]domain.OutboxJob, error)
	CompleteJob(ctx context.Context, id string, now time.Time) error
	RetryJob(ctx context.Context, id string, availableAt time.Time, lastError string, dead bool) error
	ExpireHandoffs(ctx context.Context, now time.Time, limit int) ([]domain.CustodyHandoff, error)
}

type PageRequest struct {
	Limit  int
	Offset int
	Sort   string
	Desc   bool
}

func (p PageRequest) Normalize(max int) PageRequest {
	if p.Limit < 1 {
		p.Limit = 50
	}
	if p.Limit > max {
		p.Limit = max
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

type ShipmentFilter struct {
	Page          PageRequest
	StudyID       string
	OriginSiteID  string
	DestinationID string
	State         domain.ShipmentState
	From          *time.Time
	To            *time.Time
}

type ShipmentPage struct {
	Items []domain.Shipment
	Total int
}

type SampleFilter struct {
	Page       PageRequest
	StudyID    string
	SiteID     string
	ShipmentID string
	State      domain.SampleState
	ExpiresBy  *time.Time
}

type SamplePage struct {
	Items []domain.SampleBatch
	Total int
}

type ExcursionFilter struct {
	Page       PageRequest
	ShipmentID string
	Status     domain.ExcursionStatus
	DueBefore  *time.Time
}

type ExcursionPage struct {
	Items []domain.Excursion
	Total int
}

type AuditFilter struct {
	Page       PageRequest
	EntityType string
	EntityID   string
	Actor      string
	RequestID  string
}

type AuditPage struct {
	Items []domain.AuditEvent
	Total int
}

type OperationalSummary struct {
	StudiesActive       int `json:"studies_active"`
	SamplesReady        int `json:"samples_ready"`
	SamplesInTransit    int `json:"samples_in_transit"`
	SamplesQuarantined  int `json:"samples_quarantined"`
	ContainersAvailable int `json:"containers_available"`
	ShipmentsActive     int `json:"shipments_active"`
	OpenExcursions      int `json:"open_excursions"`
	PendingHandoffs     int `json:"pending_handoffs"`
	FailedJobs          int `json:"failed_jobs"`
}

type IdempotencyRecord struct {
	Scope        string
	Key          string
	RequestHash  string
	ResponseCode int
	ResponseBody []byte
	ExpiresAt    time.Time
	CreatedAt    time.Time
}
