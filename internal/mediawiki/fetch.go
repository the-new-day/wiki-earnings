package mediawiki

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

func fetch(ctx context.Context, rawURL string) ([]byte, error) {
	const maxAttempts = 4
	var lastErr error

	for attempt := range maxAttempts {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * time.Second // 2s, 4s, 8s
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			continue
		}

		return body, nil
	}

	return nil, fmt.Errorf("after %d attempts: %w", maxAttempts, lastErr)
}
