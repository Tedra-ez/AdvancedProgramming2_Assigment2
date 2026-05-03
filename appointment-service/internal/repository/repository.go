package repository

import (
	"context"

	"appointment-service/internal/model"
)

type AppointmentRepository interface {
	Create(ctx context.Context, appointment model.Appointment) (model.Appointment, error)
	GetByID(ctx context.Context, id string) (model.Appointment, bool, error)
	List(ctx context.Context) ([]model.Appointment, error)
	UpdateStatus(ctx context.Context, id string, status model.Status) (model.Appointment, bool, error)
}
