package event

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"doctor-service/internal/model"
	"doctor-service/internal/usecase/port"

	"github.com/nats-io/nats.go"
)

type DoctorCreatedEvent struct {
	EventType   string `json:"event_type"`
	OccurredAt  string `json:"occurred_at"`
	ID          string `json:"id"`
	FullName    string `json:"full_name"`
	Specialization string `json:"specialization"`
	Email       string `json:"email"`
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

func (p *NATSPublisher) PublishDoctorCreated(ctx context.Context, doctor model.Doctor) error {
	ev := DoctorCreatedEvent{
		EventType:      "doctors.created",
		OccurredAt:     time.Now().UTC().Format(time.RFC3339),
		ID:             doctor.ID,
		FullName:       doctor.FullName,
		Specialization: doctor.Specialization,
		Email:          doctor.Email,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		log.Printf("event publish marshal failed subject=%s err=%v", ev.EventType, err)
		return nil
	}
	if err := p.nc.Publish("doctors.created", b); err != nil {
		log.Printf("event publish failed subject=%s err=%v", "doctors.created", err)
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

var _ port.DoctorEventPublisher = (*NATSPublisher)(nil)

