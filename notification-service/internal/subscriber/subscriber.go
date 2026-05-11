package subscriber

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type Handler func(subject string, data []byte)

func Run(ctx context.Context, url string, subjects []string, handler Handler) error {
	nc, err := connectWithBackoff(url, 6, time.Second)
	if err != nil {
		return err
	}
	defer nc.Close()

	for _, subj := range subjects {
		subj := subj
		if _, err := nc.Subscribe(subj, func(msg *nats.Msg) {
			handler(subj, msg.Data)
		}); err != nil {
			return fmt.Errorf("subscribe failed subject=%s: %w", subj, err)
		}
	}

	if err := nc.Flush(); err != nil {
		return fmt.Errorf("flush failed: %w", err)
	}
	if err := nc.LastError(); err != nil {
		return fmt.Errorf("nats error after subscribe: %w", err)
	}

	<-ctx.Done()

	done := make(chan struct{})
	go func() {
		_ = nc.Drain()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	return nil
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
