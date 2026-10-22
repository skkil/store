package credentials

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maxAnswerBytes = 64 << 10

// URLSource POSTs {"user": "...", "client_id": "..."} to URL and reads the
// answer with Parse.
type URLSource struct {
	URL    string
	Header http.Header
	HTTP   *http.Client
}

// Fetch implements Source.
func (s URLSource) Fetch(ctx context.Context, user, client string) (Credentials, error) {
	body, err := json.Marshal(map[string]string{"user": user, "client_id": client})
	if err != nil {
		return Credentials{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return Credentials{}, err
	}
	for name, values := range s.Header {
		req.Header[name] = values
	}
	req.Header.Set("Content-Type", "application/json")
	httpClient := s.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Credentials{}, fmt.Errorf("credentials: %w", err)
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerBytes))
	if err != nil {
		return Credentials{}, fmt.Errorf("credentials: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return Credentials{}, fmt.Errorf("credentials: %s answered %s: %s", s.URL, resp.Status, bytes.TrimSpace(answer))
	}
	return Parse(answer, client)
}
