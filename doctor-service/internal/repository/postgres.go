package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"doctor-service/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
)

type PostgresDoctorRepository struct {
	db *sql.DB
}

func NewPostgresDoctorRepository(db *sql.DB) *PostgresDoctorRepository {
	return &PostgresDoctorRepository{db: db}
}

func (r *PostgresDoctorRepository) Create(ctx context.Context, doctor model.Doctor) (model.Doctor, error) {
	if doctor.ID == "" {
		doctor.ID = fmt.Sprintf("doctor-%s", uuid.NewString())
	}

	const q = `
	INSERT INTO doctors (id, full_name, specialization, email)
VALUES ($1, $2, $3, $4)
RETURNING id, full_name, specialization, email;
`
	var out model.Doctor
	err := r.db.QueryRowContext(ctx, q, doctor.ID, doctor.FullName, doctor.Specialization, doctor.Email).
		Scan(&out.ID, &out.FullName, &out.Specialization, &out.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.Doctor{}, ErrDuplicateEmail
		}
		return model.Doctor{}, err
	}
	return out, nil
}

func (r *PostgresDoctorRepository) GetByID(ctx context.Context, id string) (model.Doctor, bool, error) {
	const q = `SELECT id, full_name, specialization, email FROM doctors WHERE id = $1;`
	var out model.Doctor
	err := r.db.QueryRowContext(ctx, q, id).Scan(&out.ID, &out.FullName, &out.Specialization, &out.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Doctor{}, false, nil
		}
		return model.Doctor{}, false, err
	}
	return out, true, nil
}

func (r *PostgresDoctorRepository) List(ctx context.Context) ([]model.Doctor, error) {
	const q = `SELECT id, full_name, specialization, email FROM doctors ORDER BY created_at ASC;`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Doctor
	for rows.Next() {
		var d model.Doctor
		if err := rows.Scan(&d.ID, &d.FullName, &d.Specialization, &d.Email); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresDoctorRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	const q = `SELECT 1 FROM doctors WHERE email = $1 LIMIT 1;`
	var one int
	err := r.db.QueryRowContext(ctx, q, email).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
