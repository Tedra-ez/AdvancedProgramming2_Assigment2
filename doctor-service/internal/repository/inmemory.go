package repository

import (
	"context"
	"fmt"
	"sync"

	"doctor-service/internal/model"
)

type InMemoryDoctorRepository struct {
	mu      sync.RWMutex
	data    map[string]model.Doctor
	byEmail map[string]string
	counter int
}

func NewInMemoryDoctorRepository() *InMemoryDoctorRepository {
	return &InMemoryDoctorRepository{
		data:    make(map[string]model.Doctor),
		byEmail: make(map[string]string),
	}
}

func (r *InMemoryDoctorRepository) Create(_ context.Context, doctor model.Doctor) (model.Doctor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	doctor.ID = fmt.Sprintf("doctor-%d", r.counter)
	r.data[doctor.ID] = doctor
	r.byEmail[doctor.Email] = doctor.ID
	return doctor, nil
}

func (r *InMemoryDoctorRepository) GetByID(_ context.Context, id string) (model.Doctor, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.data[id]
	return d, ok, nil
}

func (r *InMemoryDoctorRepository) List(_ context.Context) ([]model.Doctor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.Doctor, 0, len(r.data))
	for _, d := range r.data {
		out = append(out, d)
	}
	return out, nil
}

func (r *InMemoryDoctorRepository) ExistsByEmail(_ context.Context, email string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.byEmail[email]
	return ok, nil
}
