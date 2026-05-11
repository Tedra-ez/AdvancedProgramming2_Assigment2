package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"doctor-service/internal/model"
	"doctor-service/internal/usecase/port"

	"github.com/redis/go-redis/v9"
)

const doctorsListKey = "doctors:list"

type RedisDoctorCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisDoctorCache(client *redis.Client, ttl time.Duration) *RedisDoctorCache {
	return &RedisDoctorCache{client: client, ttl: ttl}
}

func (c *RedisDoctorCache) GetDoctor(ctx context.Context, id string) (model.Doctor, bool) {
	key := "doctor:" + id
	raw, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			log.Printf("cache get failed key=%s err=%v", key, err)
		}
		return model.Doctor{}, false
	}
	var doctor model.Doctor
	if err := json.Unmarshal(raw, &doctor); err != nil {
		log.Printf("cache decode failed key=%s err=%v", key, err)
		return model.Doctor{}, false
	}
	return doctor, true
}

func (c *RedisDoctorCache) SetDoctor(ctx context.Context, doctor model.Doctor) {
	key := "doctor:" + doctor.ID
	c.setJSON(ctx, key, doctor)
}

func (c *RedisDoctorCache) GetDoctors(ctx context.Context) ([]model.Doctor, bool) {
	raw, err := c.client.Get(ctx, doctorsListKey).Bytes()
	if err != nil {
		if err != redis.Nil {
			log.Printf("cache get failed key=%s err=%v", doctorsListKey, err)
		}
		return nil, false
	}
	var doctors []model.Doctor
	if err := json.Unmarshal(raw, &doctors); err != nil {
		log.Printf("cache decode failed key=%s err=%v", doctorsListKey, err)
		return nil, false
	}
	return doctors, true
}

func (c *RedisDoctorCache) SetDoctors(ctx context.Context, doctors []model.Doctor) {
	c.setJSON(ctx, doctorsListKey, doctors)
}

func (c *RedisDoctorCache) DeleteDoctorsList(ctx context.Context) {
	if err := c.client.Del(ctx, doctorsListKey).Err(); err != nil {
		log.Printf("cache delete failed key=%s err=%v", doctorsListKey, err)
	}
}

func (c *RedisDoctorCache) Close() error {
	return c.client.Close()
}

func (c *RedisDoctorCache) setJSON(ctx context.Context, key string, value any) {
	b, err := json.Marshal(value)
	if err != nil {
		log.Printf("cache encode failed key=%s err=%v", key, err)
		return
	}
	if err := c.client.Set(ctx, key, b, c.ttl).Err(); err != nil {
		log.Printf("cache set failed key=%s err=%v", key, err)
	}
}

var _ port.DoctorCacheRepository = (*RedisDoctorCache)(nil)
