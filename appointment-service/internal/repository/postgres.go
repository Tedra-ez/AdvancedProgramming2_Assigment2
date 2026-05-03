package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"appointment-service/internal/model"

	"github.com/google/uuid"
)

type PostgresAppointmentRepository struct {
	db *sql.DB
}

func NewPostgresAppointmentRepository(db *sql.DB) *PostgresAppointmentRepository {
	return &PostgresAppointmentRepository{db: db}
}

func (r *PostgresAppointmentRepository) Create(ctx context.Context, appointment model.Appointment) (model.Appointment, error) {
	if appointment.ID == "" {
		appointment.ID = fmt.Sprintf("appointment-%s", uuid.NewString())
	}

	const q = `
INSERT INTO appointments (id, title, description, doctor_id, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, title, description, doctor_id, status, created_at, updated_at;
`
	var out model.Appointment
	err := r.db.QueryRowContext(ctx, q,
		appointment.ID,
		appointment.Title,
		appointment.Description,
		appointment.DoctorID,
		string(appointment.Status),
	).Scan(&out.ID, &out.Title, &out.Description, &out.DoctorID, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return model.Appointment{}, err
	}
	return out, nil
}

func (r *PostgresAppointmentRepository) GetByID(ctx context.Context, id string) (model.Appointment, bool, error) {
	const q = `
SELECT id, title, description, doctor_id, status, created_at, updated_at
FROM appointments
WHERE id = $1;
`
	var out model.Appointment
	err := r.db.QueryRowContext(ctx, q, id).
		Scan(&out.ID, &out.Title, &out.Description, &out.DoctorID, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Appointment{}, false, nil
		}
		return model.Appointment{}, false, err
	}
	return out, true, nil
}

func (r *PostgresAppointmentRepository) List(ctx context.Context) ([]model.Appointment, error) {
	const q = `
SELECT id, title, description, doctor_id, status, created_at, updated_at
FROM appointments
ORDER BY created_at ASC;
`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Appointment
	for rows.Next() {
		var a model.Appointment
		if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.DoctorID, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresAppointmentRepository) UpdateStatus(ctx context.Context, id string, status model.Status) (model.Appointment, bool, error) {
	const q = `
UPDATE appointments
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, title, description, doctor_id, status, created_at, updated_at;
`
	var out model.Appointment
	err := r.db.QueryRowContext(ctx, q, id, string(status)).
		Scan(&out.ID, &out.Title, &out.Description, &out.DoctorID, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Appointment{}, false, nil
		}
		return model.Appointment{}, false, err
	}
	return out, true, nil
}

