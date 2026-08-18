package service

import (
	"context"

	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/domain"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/identity"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/repository"
	"github.com/zhanglei10281852-gif/coldchain-custody-base/internal/requestmeta"
)

type CatalogService struct{ dependencies }

func (s *CatalogService) CreateStudy(ctx context.Context, study domain.Study) (domain.Study, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return domain.Study{}, err
	}
	now := s.clock.Now()
	study.ID = identity.New("study")
	study.Code = domain.NormalizeCode(study.Code)
	if err := domain.ValidateBusinessCode("code", study.Code); err != nil {
		return domain.Study{}, err
	}
	study.Status = domain.StudyDraft
	study.Version = 1
	study.CreatedAt, study.UpdatedAt = now, now
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		if err := tx.InsertStudy(ctx, study); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, "study_created", "study", study.ID, "success", nil)
	})
	return study, err
}

func (s *CatalogService) ActivateStudy(ctx context.Context, studyID string) (domain.Study, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return domain.Study{}, err
	}
	var result domain.Study
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		study, err := tx.GetStudy(ctx, studyID)
		if err != nil {
			return err
		}
		if study.Status != domain.StudyDraft {
			return domain.TransitionError{Entity: "study", From: string(study.Status), To: string(domain.StudyActive)}
		}
		before := study.Version
		study.Status = domain.StudyActive
		study.UpdatedAt = s.clock.Now()
		if err := tx.UpdateStudy(ctx, study, before); err != nil {
			return err
		}
		result = study
		return s.audit.Record(ctx, tx, "study_activated", "study", study.ID, "success", nil)
	})
	return result, err
}

func (s *CatalogService) CreateSite(ctx context.Context, site domain.Site) (domain.Site, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return domain.Site{}, err
	}
	now := s.clock.Now()
	site.ID = identity.New("site")
	site.Code = domain.NormalizeCode(site.Code)
	if err := domain.ValidateBusinessCode("code", site.Code); err != nil {
		return domain.Site{}, err
	}
	site.Status = domain.SiteActive
	site.Version = 1
	site.CreatedAt, site.UpdatedAt = now, now
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		if err := tx.InsertSite(ctx, site); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, "site_created", "site", site.ID, "success", nil)
	})
	return site, err
}

func (s *CatalogService) CreateContainer(ctx context.Context, container domain.Container) (domain.Container, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return domain.Container{}, err
	}
	now := s.clock.Now()
	container.ID = identity.New("box")
	container.SerialNumber = domain.NormalizeCode(container.SerialNumber)
	if err := domain.ValidateBusinessCode("serial_number", container.SerialNumber); err != nil {
		return domain.Container{}, err
	}
	container.State = domain.ContainerAvailable
	container.Version = 1
	container.CreatedAt, container.UpdatedAt = now, now
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		if err := tx.InsertContainer(ctx, container); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, "container_created", "container", container.ID, "success", nil)
	})
	return container, err
}

func (s *CatalogService) RegisterSample(ctx context.Context, batch domain.SampleBatch) (domain.SampleBatch, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return domain.SampleBatch{}, err
	}
	now := s.clock.Now()
	batch.ID = identity.New("sample")
	batch.State = domain.SampleRegistered
	batch.Version = 1
	batch.CreatedAt, batch.UpdatedAt = now, now
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		if err := tx.InsertSampleBatch(ctx, batch); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, "sample_registered", "sample_batch", batch.ID, "success", nil)
	})
	return batch, err
}

func (s *CatalogService) MarkSampleReady(ctx context.Context, batchID string) (domain.SampleBatch, error) {
	principal, _ := principalOrEmpty(ctx)
	if err := requireRole(principal, domain.RoleOperations); err != nil {
		return domain.SampleBatch{}, err
	}
	var result domain.SampleBatch
	err := s.store.WithTx(ctx, func(tx repository.Tx) error {
		batch, err := tx.GetSampleBatch(ctx, batchID)
		if err != nil {
			return err
		}
		if err := batch.Transition(domain.SampleReady, s.clock.Now()); err != nil {
			return err
		}
		if err := tx.UpdateSampleBatch(ctx, batch, batch.Version); err != nil {
			return err
		}
		result = batch
		return s.audit.Record(ctx, tx, "sample_ready", "sample_batch", batch.ID, "success", nil)
	})
	return result, err
}

func principalOrEmpty(ctx context.Context) (domain.Principal, bool) {
	principal, ok := requestmeta.Principal(ctx)
	return principal, ok
}
