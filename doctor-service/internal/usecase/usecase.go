package usecase

import (
	"context"
	"errors"
	"strings"

	"doctor-service/internal/model"
	"doctor-service/internal/repository"
	"doctor-service/internal/usecase/port"
)

var (
	ErrFullNameRequired = errors.New("full_name is required")
	ErrEmailRequired    = errors.New("email is required")
	ErrEmailNotUnique   = errors.New("email must be unique")
)

type DoctorUseCase struct {
	repo      repository.DoctorRepository
	publisher port.DoctorEventPublisher
	cache     port.DoctorCacheRepository
}

func NewDoctorUseCase(repo repository.DoctorRepository, publisher port.DoctorEventPublisher, caches ...port.DoctorCacheRepository) *DoctorUseCase {
	var cache port.DoctorCacheRepository = port.NoopDoctorCacheRepository{}
	if len(caches) > 0 && caches[0] != nil {
		cache = caches[0]
	}
	return &DoctorUseCase{repo: repo, publisher: publisher, cache: cache}
}

func (u *DoctorUseCase) Create(ctx context.Context, doctor model.Doctor) (model.Doctor, error) {
	if strings.TrimSpace(doctor.FullName) == "" {
		return model.Doctor{}, ErrFullNameRequired
	}
	if strings.TrimSpace(doctor.Email) == "" {
		return model.Doctor{}, ErrEmailRequired
	}
	exists, err := u.repo.ExistsByEmail(ctx, doctor.Email)
	if err != nil {
		return model.Doctor{}, err
	}
	if exists {
		return model.Doctor{}, ErrEmailNotUnique
	}
	created, err := u.repo.Create(ctx, doctor)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return model.Doctor{}, ErrEmailNotUnique
		}
		return model.Doctor{}, err
	}

	u.cache.SetDoctor(ctx, created)
	u.cache.DeleteDoctorsList(ctx)
	if u.publisher != nil {
		_ = u.publisher.PublishDoctorCreated(ctx, created)
	}
	return created, nil
}

func (u *DoctorUseCase) GetByID(ctx context.Context, id string) (model.Doctor, bool, error) {
	if cached, ok := u.cache.GetDoctor(ctx, id); ok {
		return cached, true, nil
	}
	doctor, ok, err := u.repo.GetByID(ctx, id)
	if err != nil || !ok {
		return doctor, ok, err
	}
	u.cache.SetDoctor(ctx, doctor)
	return doctor, true, nil
}

func (u *DoctorUseCase) List(ctx context.Context) ([]model.Doctor, error) {
	if cached, ok := u.cache.GetDoctors(ctx); ok {
		return cached, nil
	}
	doctors, err := u.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	u.cache.SetDoctors(ctx, doctors)
	return doctors, nil
}
