package port

import (
	"context"

	"doctor-service/internal/model"
)

type DoctorCacheRepository interface {
	GetDoctor(ctx context.Context, id string) (model.Doctor, bool)
	SetDoctor(ctx context.Context, doctor model.Doctor)
	GetDoctors(ctx context.Context) ([]model.Doctor, bool)
	SetDoctors(ctx context.Context, doctors []model.Doctor)
	DeleteDoctorsList(ctx context.Context)
	Close() error
}

type NoopDoctorCacheRepository struct{}

func (NoopDoctorCacheRepository) GetDoctor(context.Context, string) (model.Doctor, bool) {
	return model.Doctor{}, false
}
func (NoopDoctorCacheRepository) SetDoctor(context.Context, model.Doctor) {}
func (NoopDoctorCacheRepository) GetDoctors(context.Context) ([]model.Doctor, bool) {
	return nil, false
}
func (NoopDoctorCacheRepository) SetDoctors(context.Context, []model.Doctor) {}
func (NoopDoctorCacheRepository) DeleteDoctorsList(context.Context)          {}
func (NoopDoctorCacheRepository) Close() error                               { return nil }
