package jobqueue

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	maxAttempts    = 3
	idempotencyTTL = 24 * time.Hour
)

type Config struct {
	RedisURL       string
	GatewayURL     string
	WorkerPoolSize int
}

type Manager struct {
	queue      chan Job
	store      IdempotencyStore
	gatewayURL string
	client     *http.Client
	workers    int
	wg         sync.WaitGroup
}

type Job struct {
	IdempotencyKey string `json:"idempotency_key"`
	AppointmentID  string `json:"appointment_id"`
	DoctorID       string `json:"doctor_id"`
	OccurredAt     string `json:"occurred_at"`
	Channel        string `json:"channel"`
	Recipient      string `json:"recipient"`
	Message        string `json:"message"`
}

type gatewayRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Channel        string `json:"channel"`
	Recipient      string `json:"recipient"`
	Message        string `json:"message"`
}

type jobLog struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	JobID   string `json:"job_id"`
	Attempt int    `json:"attempt"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

func New(cfg Config) *Manager {
	workers := cfg.WorkerPoolSize
	if workers <= 0 {
		workers = 3
	}
	return &Manager{
		queue:      make(chan Job, workers*10),
		store:      newStore(cfg.RedisURL),
		gatewayURL: strings.TrimRight(cfg.GatewayURL, "/"),
		client:     &http.Client{Timeout: 5 * time.Second},
		workers:    workers,
	}
}

func (m *Manager) Start(ctx context.Context) {
	for i := 0; i < m.workers; i++ {
		m.wg.Add(1)
		go m.worker(ctx)
	}
}

func (m *Manager) Close() {
	m.store.Close()
	m.wg.Wait()
}

func (m *Manager) EnqueueFromEvent(ctx context.Context, event map[string]any) {
	if stringField(event, "new_status") != "done" {
		return
	}

	job := buildJob(event)
	if job.IdempotencyKey == "" || job.AppointmentID == "" {
		writeJobLog(os.Stderr, "error", job.IdempotencyKey, 0, "dead_letter", "missing required event fields")
		return
	}
	if job.DoctorID == "" {
		writeJobLog(os.Stderr, "error", job.IdempotencyKey, 0, "dead_letter", "missing doctor_id in status event")
		return
	}

	reserved, err := m.store.Reserve(ctx, job.IdempotencyKey)
	if err != nil {
		log.Printf("job idempotency reserve failed key=%s err=%v", job.IdempotencyKey, err)
		return
	}
	if !reserved {
		writeJobLog(os.Stdout, "info", job.IdempotencyKey, 0, "duplicate", "")
		return
	}

	select {
	case m.queue <- job:
		writeJobLog(os.Stdout, "info", job.IdempotencyKey, 0, "enqueued", "")
	default:
		writeJobLog(os.Stderr, "error", job.IdempotencyKey, 0, "dead_letter", "job queue is full")
	}
}

func (m *Manager) worker(ctx context.Context) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-m.queue:
			m.process(ctx, job)
		}
	}
}

func (m *Manager) process(ctx context.Context, job Job) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		writeJobLog(os.Stdout, "info", job.IdempotencyKey, attempt, "processing", "")
		err := m.callGateway(ctx, job)
		if err == nil {
			if markErr := m.store.MarkDone(ctx, job.IdempotencyKey); markErr != nil {
				log.Printf("job idempotency mark done failed key=%s err=%v", job.IdempotencyKey, markErr)
			}
			writeJobLog(os.Stdout, "info", job.IdempotencyKey, attempt, "success", "")
			return
		}
		if attempt == maxAttempts {
			writeJobLog(os.Stderr, "error", job.IdempotencyKey, attempt, "dead_letter", err.Error())
			return
		}
		writeJobLog(os.Stdout, "warn", job.IdempotencyKey, attempt, "retry", err.Error())
		if !sleep(ctx, time.Duration(1<<uint(attempt-1))*time.Second) {
			return
		}
	}
}

func (m *Manager) callGateway(ctx context.Context, job Job) error {
	if m.gatewayURL == "" {
		return errors.New("gateway URL is empty")
	}
	body := gatewayRequest{
		IdempotencyKey: job.IdempotencyKey,
		Channel:        job.Channel,
		Recipient:      job.Recipient,
		Message:        job.Message,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.gatewayURL+"/notify", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("gateway returned status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func buildJob(event map[string]any) Job {
	eventType := stringField(event, "event_type")
	id := stringField(event, "id")
	occurredAt := stringField(event, "occurred_at")
	doctorID := stringField(event, "doctor_id")
	key := idempotencyKey(eventType, id, occurredAt)
	return Job{
		IdempotencyKey: key,
		AppointmentID:  id,
		DoctorID:       doctorID,
		OccurredAt:     occurredAt,
		Channel:        "email",
		Recipient:      "patient@clinic.kz",
		Message:        fmt.Sprintf("Your appointment %s with doctor %s is complete.", id, doctorID),
	}
}

func idempotencyKey(eventType, id, occurredAt string) string {
	if eventType == "" || id == "" || occurredAt == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(eventType + ":" + id + ":" + occurredAt))
	return hex.EncodeToString(sum[:])
}

func stringField(event map[string]any, key string) string {
	v, _ := event[key].(string)
	return v
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func writeJobLog(w io.Writer, level, jobID string, attempt int, statusText, errText string) {
	entry := jobLog{
		Time:    time.Now().UTC().Format(time.RFC3339),
		Level:   level,
		JobID:   jobID,
		Attempt: attempt,
		Status:  statusText,
		Error:   errText,
	}
	b, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(w, `{"time":"%s","level":"error","job_id":"%s","attempt":%d,"status":"dead_letter","error":"marshal log failed"}`+"\n", time.Now().UTC().Format(time.RFC3339), jobID, attempt)
		return
	}
	fmt.Fprintln(w, string(b))
}

type IdempotencyStore interface {
	Reserve(ctx context.Context, key string) (bool, error)
	MarkDone(ctx context.Context, key string) error
	Close()
}

type redisStore struct {
	client *redis.Client
}

func newStore(redisURL string) IdempotencyStore {
	if redisURL == "" {
		log.Printf("warning: REDIS_URL not set, using in-memory idempotency store")
		return newMemoryStore()
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("warning: invalid REDIS_URL, using in-memory idempotency store: %v", err)
		return newMemoryStore()
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		log.Printf("warning: Redis unavailable, using in-memory idempotency store: %v", err)
		return newMemoryStore()
	}
	return &redisStore{client: client}
}

func (s *redisStore) Reserve(ctx context.Context, key string) (bool, error) {
	return s.client.SetNX(ctx, "job:"+key, "processing", idempotencyTTL).Result()
}

func (s *redisStore) MarkDone(ctx context.Context, key string) error {
	return s.client.Set(ctx, "job:"+key, "done", idempotencyTTL).Err()
}

func (s *redisStore) Close() {
	_ = s.client.Close()
}

type memoryStore struct {
	mu   sync.Mutex
	data map[string]string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{data: make(map[string]string)}
}

func (s *memoryStore) Reserve(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; ok {
		return false, nil
	}
	s.data[key] = "processing"
	return true, nil
}

func (s *memoryStore) MarkDone(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = "done"
	return nil
}

func (s *memoryStore) Close() {}
