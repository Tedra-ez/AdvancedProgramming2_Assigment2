package repository

import (
	"context"
	"errors"

	"doctor-service/internal/model"
)

type DoctorRepository interface {
	Create(ctx context.Context, doctor model.Doctor) (model.Doctor, error)
	GetByID(ctx context.Context, id string) (model.Doctor, bool, error)
	List(ctx context.Context) ([]model.Doctor, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

var ErrDuplicateEmail = errors.New("duplicate email")
