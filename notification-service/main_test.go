package main

import (
	"bytes"
	"encoding/json"
	"testing"

	eventlogger "notification-service/internal/logger"
)

func TestLogEventIncludesSubjectAndEvent(t *testing.T) {
	data := []byte(`{"event_type":"doctors.created","id":"doctor-1","full_name":"Dr. Aisha Seitkali"}`)
	var buf bytes.Buffer

	event, err := eventlogger.LogEvent(&buf, "doctors.created", data)
	if err != nil {
		t.Fatalf("LogEvent returned error: %v", err)
	}
	if event["event_type"] != "doctors.created" || event["id"] != "doctor-1" {
		t.Fatalf("event = %#v, want deserialized payload", event)
	}

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if got["time"] == "" {
		t.Fatalf("time is empty")
	}
	if got["subject"] != "doctors.created" {
		t.Fatalf("subject = %v, want doctors.created", got["subject"])
	}

	event, ok := got["event"].(map[string]any)
	if !ok {
		t.Fatalf("event = %#v, want object", got["event"])
	}
	if event["event_type"] != "doctors.created" || event["id"] != "doctor-1" {
		t.Fatalf("event = %#v, want deserialized payload", event)
	}
}

func TestLogEventRejectsInvalidJSON(t *testing.T) {
	_, err := eventlogger.LogEvent(&bytes.Buffer{}, "doctors.created", []byte(`not-json`))
	if err == nil {
		t.Fatal("LogEvent returned nil error for invalid JSON")
	}
}
