package port

import (
	"context"

	"doctor-service/internal/model"
)

type DoctorEventPublisher interface {
	PublishDoctorCreated(ctx context.Context, doctor model.Doctor) error
	Close() error
}

type NoopDoctorEventPublisher struct{}

func (NoopDoctorEventPublisher) PublishDoctorCreated(context.Context, model.Doctor) error { return nil }
func (NoopDoctorEventPublisher) Close() error                                            { return nil }

