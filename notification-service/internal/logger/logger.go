package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

func LogEvent(w io.Writer, subject string, data []byte) (map[string]any, error) {
	var ev map[string]any
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil, err
	}

	out := map[string]any{
		"time":    time.Now().UTC().Format(time.RFC3339),
		"subject": subject,
		"event":   ev,
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(w, string(b))
	return ev, nil
}
