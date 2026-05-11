package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

type notifyRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Channel        string `json:"channel"`
	Recipient      string `json:"recipient"`
	Message        string `json:"message"`
}

type gateway struct {
	mu   sync.Mutex
	seen map[string]struct{}
	rnd  *rand.Rand
}

func main() {
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8080"
	}

	g := &gateway{
		seen: make(map[string]struct{}),
		rnd:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /notify", g.notify)

	addr := ":" + port
	log.Printf("mock-gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("mock-gateway failed: %v", err)
	}
}

func (g *gateway) notify(w http.ResponseWriter, r *http.Request) {
	var req notifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	g.logRequest(req)

	g.mu.Lock()
	transientFailure := g.rnd.Intn(100) < 20
	if transientFailure {
		g.mu.Unlock()
		http.Error(w, "transient failure", http.StatusServiceUnavailable)
		return
	}

	status := "accepted"
	if _, ok := g.seen[req.IdempotencyKey]; ok {
		status = "duplicate"
	} else {
		g.seen[req.IdempotencyKey] = struct{}{}
	}
	g.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func (g *gateway) logRequest(req notifyRequest) {
	entry := map[string]any{
		"time":            time.Now().UTC().Format(time.RFC3339),
		"idempotency_key": req.IdempotencyKey,
		"channel":         req.Channel,
		"recipient":       req.Recipient,
		"message":         req.Message,
	}
	b, err := json.Marshal(entry)
	if err != nil {
		log.Printf("mock-gateway: log marshal failed: %v", err)
		return
	}
	log.Println(string(b))
}
