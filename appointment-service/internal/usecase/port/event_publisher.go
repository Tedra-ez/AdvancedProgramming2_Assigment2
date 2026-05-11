package port

import (
	"context"

	"appointment-service/internal/model"
)

type AppointmentEventPublisher interface {
	PublishAppointmentCreated(ctx context.Context, appointment model.Appointment) error
	PublishAppointmentStatusUpdated(ctx context.Context, id string, doctorID string, oldStatus, newStatus model.Status) error
	Close() error
}

type NoopAppointmentEventPublisher struct{}

func (NoopAppointmentEventPublisher) PublishAppointmentCreated(context.Context, model.Appointment) error {
	return nil
}
func (NoopAppointmentEventPublisher) PublishAppointmentStatusUpdated(context.Context, string, string, model.Status, model.Status) error {
	return nil
}
func (NoopAppointmentEventPublisher) Close() error { return nil }
