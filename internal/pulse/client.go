package pulse

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ListenSSE opens an SSE stream to url and invokes handler for each event.
// It returns when the connection ends or ctx is cancelled.
func ListenSSE(ctx context.Context, url string, handler EventHandler) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pulse: status %s", resp.Status)
	}

	reader := bufio.NewReader(resp.Body)
	var eventType string
	var dataBuffer strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if dataBuffer.Len() > 0 {
				if handler != nil {
					handler(eventType, dataBuffer.String())
				}
				dataBuffer.Reset()
				eventType = ""
			}
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataBuffer.WriteString(strings.TrimPrefix(line, "data: "))
		} else {
			dataBuffer.WriteString(line)
		}
	}
}

// FormatSSE builds a single SSE message.
func FormatSSE(eventType string, data []byte) string {
	return fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(data))
}
