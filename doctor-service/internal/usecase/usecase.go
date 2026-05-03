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
}

func NewDoctorUseCase(repo repository.DoctorRepository, publisher port.DoctorEventPublisher) *DoctorUseCase {
	return &DoctorUseCase{repo: repo, publisher: publisher}
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

	if u.publisher != nil {
		_ = u.publisher.PublishDoctorCreated(ctx, created)
	}
	return created, nil
}

func (u *DoctorUseCase) GetByID(ctx context.Context, id string) (model.Doctor, bool, error) {
	return u.repo.GetByID(ctx, id)
}

func (u *DoctorUseCase) List(ctx context.Context) ([]model.Doctor, error) {
	return u.repo.List(ctx)
}
