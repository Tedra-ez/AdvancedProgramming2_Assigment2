package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		log.Fatalf("notification-service: missing NATS_URL")
	}

	nc, err := connectWithBackoff(natsURL, 6, 1*time.Second)
	if err != nil {
		log.Fatalf("notification-service: %v", err)
	}
	defer nc.Close()

	subjects := []string{
		"doctors.created",
		"appointments.created",
		"appointments.status_updated",
	}

	for _, subj := range subjects {
		subj := subj
		_, err := nc.Subscribe(subj, func(msg *nats.Msg) {
			var ev map[string]any
			if err := json.Unmarshal(msg.Data, &ev); err != nil {
				log.Printf("notification-service: invalid json subject=%s err=%v", subj, err)
				return
			}
			out := map[string]any{
				"time":    time.Now().UTC().Format(time.RFC3339),
				"subject": subj,
				"event":   ev,
			}
			b, err := json.Marshal(out)
			if err != nil {
				log.Printf("notification-service: marshal log failed subject=%s err=%v", subj, err)
				return
			}
			fmt.Println(string(b))
		})
		if err != nil {
			log.Fatalf("notification-service: subscribe failed subject=%s err=%v", subj, err)
		}
	}

	if err := nc.Flush(); err != nil {
		log.Fatalf("notification-service: flush failed: %v", err)
	}
	if err := nc.LastError(); err != nil {
		log.Fatalf("notification-service: nats error after subscribe: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	drainDone := make(chan struct{})
	go func() {
		_ = nc.Drain()
		close(drainDone)
	}()

	select {
	case <-drainDone:
	case <-time.After(5 * time.Second):
	}
}

func connectWithBackoff(url string, maxAttempts int, initialDelay time.Duration) (*nats.Conn, error) {
	delay := initialDelay
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		nc, err := nats.Connect(url)
		if err == nil {
			return nc, nil
		}
		lastErr = err
		if attempt == maxAttempts {
			break
		}
		time.Sleep(delay)
		delay *= 2
	}
	if lastErr == nil {
		lastErr = errors.New("unknown connection error")
	}
	return nil, fmt.Errorf("cannot connect to NATS after %d attempts: %w", maxAttempts, lastErr)
}
