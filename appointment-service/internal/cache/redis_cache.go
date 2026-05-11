package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"appointment-service/internal/model"
	"appointment-service/internal/usecase/port"

	"github.com/redis/go-redis/v9"
)

const appointmentsListKey = "appointments:list"

type RedisAppointmentCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisAppointmentCache(client *redis.Client, ttl time.Duration) *RedisAppointmentCache {
	return &RedisAppointmentCache{client: client, ttl: ttl}
}

func (c *RedisAppointmentCache) GetAppointment(ctx context.Context, id string) (model.Appointment, bool) {
	key := "appointment:" + id
	raw, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			log.Printf("cache get failed key=%s err=%v", key, err)
		}
		return model.Appointment{}, false
	}
	var appointment model.Appointment
	if err := json.Unmarshal(raw, &appointment); err != nil {
		log.Printf("cache decode failed key=%s err=%v", key, err)
		return model.Appointment{}, false
	}
	return appointment, true
}

func (c *RedisAppointmentCache) SetAppointment(ctx context.Context, appointment model.Appointment) {
	key := "appointment:" + appointment.ID
	c.setJSON(ctx, key, appointment)
}

func (c *RedisAppointmentCache) DeleteAppointment(ctx context.Context, id string) {
	key := "appointment:" + id
	if err := c.client.Del(ctx, key).Err(); err != nil {
		log.Printf("cache delete failed key=%s err=%v", key, err)
	}
}

func (c *RedisAppointmentCache) GetAppointments(ctx context.Context) ([]model.Appointment, bool) {
	raw, err := c.client.Get(ctx, appointmentsListKey).Bytes()
	if err != nil {
		if err != redis.Nil {
			log.Printf("cache get failed key=%s err=%v", appointmentsListKey, err)
		}
		return nil, false
	}
	var appointments []model.Appointment
	if err := json.Unmarshal(raw, &appointments); err != nil {
		log.Printf("cache decode failed key=%s err=%v", appointmentsListKey, err)
		return nil, false
	}
	return appointments, true
}

func (c *RedisAppointmentCache) SetAppointments(ctx context.Context, appointments []model.Appointment) {
	c.setJSON(ctx, appointmentsListKey, appointments)
}

func (c *RedisAppointmentCache) DeleteAppointmentsList(ctx context.Context) {
	if err := c.client.Del(ctx, appointmentsListKey).Err(); err != nil {
		log.Printf("cache delete failed key=%s err=%v", appointmentsListKey, err)
	}
}

func (c *RedisAppointmentCache) Close() error {
	return c.client.Close()
}

func (c *RedisAppointmentCache) setJSON(ctx context.Context, key string, value any) {
	b, err := json.Marshal(value)
	if err != nil {
		log.Printf("cache encode failed key=%s err=%v", key, err)
		return
	}
	if err := c.client.Set(ctx, key, b, c.ttl).Err(); err != nil {
		log.Printf("cache set failed key=%s err=%v", key, err)
	}
}

var _ port.AppointmentCacheRepository = (*RedisAppointmentCache)(nil)
