package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"appointment-service/internal/model"
)

type InMemoryAppointmentRepository struct {
	mu      sync.RWMutex
	data    map[string]model.Appointment
	counter int
}

func NewInMemoryAppointmentRepository() *InMemoryAppointmentRepository {
	return &InMemoryAppointmentRepository{
		data: make(map[string]model.Appointment),
	}
}

func (r *InMemoryAppointmentRepository) Create(_ context.Context, appointment model.Appointment) (model.Appointment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	now := time.Now().UTC()
	appointment.ID = fmt.Sprintf("appointment-%d", r.counter)
	appointment.CreatedAt = now
	appointment.UpdatedAt = now
	r.data[appointment.ID] = appointment
	return appointment, nil
}

func (r *InMemoryAppointmentRepository) GetByID(_ context.Context, id string) (model.Appointment, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.data[id]
	return a, ok, nil
}

func (r *InMemoryAppointmentRepository) List(_ context.Context) ([]model.Appointment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.Appointment, 0, len(r.data))
	for _, a := range r.data {
		out = append(out, a)
	}
	return out, nil
}

func (r *InMemoryAppointmentRepository) UpdateStatus(_ context.Context, id string, status model.Status) (model.Appointment, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.data[id]
	if !ok {
		return model.Appointment{}, false, nil
	}
	a.Status = status
	a.UpdatedAt = time.Now().UTC()
	r.data[id] = a
	return a, true, nil
}
