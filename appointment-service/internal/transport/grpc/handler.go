package grpc

import (
	"context"
	"errors"

	"appointment-service/internal/model"
	"appointment-service/internal/usecase"
	appointmentpb "appointment-service/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	appointmentpb.UnimplementedAppointmentServiceServer
	uc *usecase.AppointmentUseCase
}

func NewHandler(uc *usecase.AppointmentUseCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) CreateAppointment(ctx context.Context, req *appointmentpb.CreateAppointmentRequest) (*appointmentpb.AppointmentResponse, error) {
	created, err := h.uc.Create(ctx, model.Appointment{
		Title:       req.GetTitle(),
		Description: req.GetDescription(),
		DoctorID:    req.GetDoctorId(),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrTitleRequired), errors.Is(err, usecase.ErrDoctorIDRequired):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, usecase.ErrDoctorNotFound):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		case errors.Is(err, usecase.ErrDoctorServiceUnavailable):
			return nil, status.Error(codes.Unavailable, "cannot validate doctor: doctor service is unavailable")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return toAppointmentResponse(created), nil
}

func (h *Handler) GetAppointment(ctx context.Context, req *appointmentpb.GetAppointmentRequest) (*appointmentpb.AppointmentResponse, error) {
	appointment, ok, err := h.uc.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "appointment not found")
	}
	return toAppointmentResponse(appointment), nil
}

func (h *Handler) ListAppointments(ctx context.Context, _ *appointmentpb.ListAppointmentsRequest) (*appointmentpb.ListAppointmentsResponse, error) {
	appointments, err := h.uc.List(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &appointmentpb.ListAppointmentsResponse{
		Appointments: make([]*appointmentpb.AppointmentResponse, 0, len(appointments)),
	}
	for _, a := range appointments {
		resp.Appointments = append(resp.Appointments, toAppointmentResponse(a))
	}
	return resp, nil
}

func (h *Handler) UpdateAppointmentStatus(ctx context.Context, req *appointmentpb.UpdateStatusRequest) (*appointmentpb.AppointmentResponse, error) {
	updated, err := h.uc.UpdateStatus(ctx, req.GetId(), model.Status(req.GetStatus()))
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidStatus), errors.Is(err, usecase.ErrInvalidStatusTransition):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, usecase.ErrAppointmentNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, usecase.ErrDoctorNotFound):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		case errors.Is(err, usecase.ErrDoctorServiceUnavailable):
			return nil, status.Error(codes.Unavailable, "cannot validate doctor: doctor service is unavailable")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return toAppointmentResponse(updated), nil
}

func toAppointmentResponse(a model.Appointment) *appointmentpb.AppointmentResponse {
	return &appointmentpb.AppointmentResponse{
		Id:          a.ID,
		Title:       a.Title,
		Description: a.Description,
		DoctorId:    a.DoctorID,
		Status:      string(a.Status),
		CreatedAt:   a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   a.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
