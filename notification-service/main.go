package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"notification-service/internal/jobqueue"
	eventlogger "notification-service/internal/logger"
	"notification-service/internal/subscriber"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		log.Fatalf("notification-service: missing NATS_URL")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	queue := jobqueue.New(jobqueue.Config{
		RedisURL:       os.Getenv("REDIS_URL"),
		GatewayURL:     gatewayURL(),
		WorkerPoolSize: workerPoolSize(),
	})
	queue.Start(ctx)
	defer queue.Close()

	subjects := []string{
		"doctors.created",
		"appointments.created",
		"appointments.status_updated",
	}

	err := subscriber.Run(ctx, natsURL, subjects, func(subject string, data []byte) {
		event, err := eventlogger.LogEvent(os.Stdout, subject, data)
		if err != nil {
			log.Printf("notification-service: invalid json subject=%s err=%v", subject, err)
			return
		}
		if subject == "appointments.status_updated" {
			queue.EnqueueFromEvent(ctx, event)
		}
	})
	if err != nil {
		log.Fatalf("notification-service: %v", err)
	}
}

func gatewayURL() string {
	url := os.Getenv("GATEWAY_URL")
	if url == "" {
		log.Printf("warning: GATEWAY_URL not set, using http://localhost:8080")
		return "http://localhost:8080"
	}
	return url
}

func workerPoolSize() int {
	raw := os.Getenv("WORKER_POOL_SIZE")
	if raw == "" {
		return 3
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("warning: invalid WORKER_POOL_SIZE=%q, using 3", raw)
		return 3
	}
	return n
}
