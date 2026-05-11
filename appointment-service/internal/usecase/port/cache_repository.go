package port

import (
	"context"

	"appointment-service/internal/model"
)

type AppointmentCacheRepository interface {
	GetAppointment(ctx context.Context, id string) (model.Appointment, bool)
	SetAppointment(ctx context.Context, appointment model.Appointment)
	DeleteAppointment(ctx context.Context, id string)
	GetAppointments(ctx context.Context) ([]model.Appointment, bool)
	SetAppointments(ctx context.Context, appointments []model.Appointment)
	DeleteAppointmentsList(ctx context.Context)
	Close() error
}

type NoopAppointmentCacheRepository struct{}

func (NoopAppointmentCacheRepository) GetAppointment(context.Context, string) (model.Appointment, bool) {
	return model.Appointment{}, false
}
func (NoopAppointmentCacheRepository) SetAppointment(context.Context, model.Appointment) {}
func (NoopAppointmentCacheRepository) DeleteAppointment(context.Context, string)         {}
func (NoopAppointmentCacheRepository) GetAppointments(context.Context) ([]model.Appointment, bool) {
	return nil, false
}
func (NoopAppointmentCacheRepository) SetAppointments(context.Context, []model.Appointment) {}
func (NoopAppointmentCacheRepository) DeleteAppointmentsList(context.Context)               {}
func (NoopAppointmentCacheRepository) Close() error                                         { return nil }
