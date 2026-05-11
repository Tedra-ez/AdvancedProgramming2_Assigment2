package event

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"appointment-service/internal/model"
	"appointment-service/internal/usecase/port"

	"github.com/nats-io/nats.go"
)

type AppointmentCreatedEvent struct {
	EventType  string `json:"event_type"`
	OccurredAt string `json:"occurred_at"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	DoctorID   string `json:"doctor_id"`
	Status     string `json:"status"`
}

type AppointmentStatusUpdatedEvent struct {
	EventType  string `json:"event_type"`
	OccurredAt string `json:"occurred_at"`
	ID         string `json:"id"`
	DoctorID   string `json:"doctor_id"`
	OldStatus  string `json:"old_status"`
	NewStatus  string `json:"new_status"`
}

type NATSPublisher struct {
	nc *nats.Conn
}

func NewNATSPublisher(url string) (*NATSPublisher, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &NATSPublisher{nc: nc}, nil
}

func (p *NATSPublisher) PublishAppointmentCreated(ctx context.Context, appointment model.Appointment) error {
	ev := AppointmentCreatedEvent{
		EventType:  "appointments.created",
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		ID:         appointment.ID,
		Title:      appointment.Title,
		DoctorID:   appointment.DoctorID,
		Status:     string(appointment.Status),
	}
	b, err := json.Marshal(ev)
	if err != nil {
		log.Printf("event publish marshal failed subject=%s err=%v", ev.EventType, err)
		return nil
	}
	if err := p.nc.Publish("appointments.created", b); err != nil {
		log.Printf("event publish failed subject=%s err=%v", "appointments.created", err)
		return nil
	}
	return nil
}

func (p *NATSPublisher) PublishAppointmentStatusUpdated(ctx context.Context, id string, doctorID string, oldStatus, newStatus model.Status) error {
	ev := AppointmentStatusUpdatedEvent{
		EventType:  "appointments.status_updated",
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		ID:         id,
		DoctorID:   doctorID,
		OldStatus:  string(oldStatus),
		NewStatus:  string(newStatus),
	}
	b, err := json.Marshal(ev)
	if err != nil {
		log.Printf("event publish marshal failed subject=%s err=%v", ev.EventType, err)
		return nil
	}
	if err := p.nc.Publish("appointments.status_updated", b); err != nil {
		log.Printf("event publish failed subject=%s err=%v", "appointments.status_updated", err)
		return nil
	}
	return nil
}

func (p *NATSPublisher) Close() error {
	if p.nc == nil {
		return nil
	}
	p.nc.Close()
	return nil
}

var _ port.AppointmentEventPublisher = (*NATSPublisher)(nil)
