package grpc

import (
	"context"
	"errors"

	"doctor-service/internal/model"
	"doctor-service/internal/usecase"
	doctorpb "doctor-service/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	doctorpb.UnimplementedDoctorServiceServer
	uc *usecase.DoctorUseCase
}

func NewHandler(uc *usecase.DoctorUseCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) CreateDoctor(ctx context.Context, req *doctorpb.CreateDoctorRequest) (*doctorpb.DoctorResponse, error) {
	created, err := h.uc.Create(ctx, model.Doctor{
		FullName:       req.GetFullName(),
		Specialization: req.GetSpecialization(),
		Email:          req.GetEmail(),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrFullNameRequired), errors.Is(err, usecase.ErrEmailRequired):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, usecase.ErrEmailNotUnique):
			return nil, status.Error(codes.AlreadyExists, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return toDoctorResponse(created), nil
}

func (h *Handler) GetDoctor(ctx context.Context, req *doctorpb.GetDoctorRequest) (*doctorpb.DoctorResponse, error) {
	doctor, ok, err := h.uc.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "doctor not found")
	}
	return toDoctorResponse(doctor), nil
}

func (h *Handler) ListDoctors(ctx context.Context, _ *doctorpb.ListDoctorsRequest) (*doctorpb.ListDoctorsResponse, error) {
	doctors, err := h.uc.List(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &doctorpb.ListDoctorsResponse{
		Doctors: make([]*doctorpb.DoctorResponse, 0, len(doctors)),
	}
	for _, d := range doctors {
		resp.Doctors = append(resp.Doctors, toDoctorResponse(d))
	}
	return resp, nil
}

func toDoctorResponse(d model.Doctor) *doctorpb.DoctorResponse {
	return &doctorpb.DoctorResponse{
		Id:             d.ID,
		FullName:       d.FullName,
		Specialization: d.Specialization,
		Email:          d.Email,
	}
}
