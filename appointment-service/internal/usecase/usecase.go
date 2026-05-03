package usecase

import (
	"context"
	"errors"
	"strings"

	"appointment-service/internal/client"
	"appointment-service/internal/model"
	"appointment-service/internal/repository"
	"appointment-service/internal/usecase/port"
)

var (
	ErrTitleRequired            = errors.New("title is required")
	ErrDoctorIDRequired         = errors.New("doctor_id is required")
	ErrDoctorNotFound           = errors.New("doctor does not exist")
	ErrDoctorServiceUnavailable = errors.New("doctor service is unavailable")
	ErrInvalidStatus            = errors.New("invalid status: allowed values are new, in_progress, done")
	ErrAppointmentNotFound      = errors.New("appointment not found")
	ErrInvalidStatusTransition  = errors.New("status transition from done to new is not allowed")
)

type AppointmentUseCase struct {
	repo         repository.AppointmentRepository
	doctorClient client.DoctorClient
	publisher    port.AppointmentEventPublisher
}

func NewAppointmentUseCase(repo repository.AppointmentRepository, doctorClient client.DoctorClient, publisher port.AppointmentEventPublisher) *AppointmentUseCase {
	return &AppointmentUseCase{
		repo:         repo,
		doctorClient: doctorClient,
		publisher:    publisher,
	}
}

func (u *AppointmentUseCase) Create(ctx context.Context, appointment model.Appointment) (model.Appointment, error) {
	if strings.TrimSpace(appointment.Title) == "" {
		return model.Appointment{}, ErrTitleRequired
	}
	if strings.TrimSpace(appointment.DoctorID) == "" {
		return model.Appointment{}, ErrDoctorIDRequired
	}

	exists, err := u.doctorClient.Exists(ctx, appointment.DoctorID)
	if err != nil {
		return model.Appointment{}, ErrDoctorServiceUnavailable
	}
	if !exists {
		return model.Appointment{}, ErrDoctorNotFound
	}

	appointment.Status = model.StatusNew
	created, err := u.repo.Create(ctx, appointment)
	if err != nil {
		return model.Appointment{}, err
	}

	if u.publisher != nil {
		_ = u.publisher.PublishAppointmentCreated(ctx, created)
	}
	return created, nil
}

func (u *AppointmentUseCase) GetByID(ctx context.Context, id string) (model.Appointment, bool, error) {
	return u.repo.GetByID(ctx, id)
}

func (u *AppointmentUseCase) List(ctx context.Context) ([]model.Appointment, error) {
	return u.repo.List(ctx)
}

func (u *AppointmentUseCase) UpdateStatus(ctx context.Context, id string, status model.Status) (model.Appointment, error) {
	if !model.IsValidStatus(status) {
		return model.Appointment{}, ErrInvalidStatus
	}
	current, ok, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return model.Appointment{}, err
	}
	if !ok {
		return model.Appointment{}, ErrAppointmentNotFound
	}
	if current.Status == model.StatusDone && status == model.StatusNew {
		return model.Appointment{}, ErrInvalidStatusTransition
	}

	exists, err := u.doctorClient.Exists(ctx, current.DoctorID)
	if err != nil {
		return model.Appointment{}, ErrDoctorServiceUnavailable
	}
	if !exists {
		return model.Appointment{}, ErrDoctorNotFound
	}

	oldStatus := current.Status
	updated, ok, err := u.repo.UpdateStatus(ctx, id, status)
	if err != nil {
		return model.Appointment{}, err
	}
	if !ok {
		return model.Appointment{}, ErrAppointmentNotFound
	}

	if u.publisher != nil {
		_ = u.publisher.PublishAppointmentStatusUpdated(ctx, updated.ID, oldStatus, updated.Status)
	}
	return updated, nil
}
