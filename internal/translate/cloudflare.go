package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

var ErrUnsupportedLanguage = errors.New("translate: unsupported language")

const (
	cloudflareModel = "@cf/meta/m2m100-1.2b"

	cloudflareEndpoint = "https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s"

	cloudflareTimeout  = 30 * time.Second
	cloudflareAttempts = 3
)

var cloudflareLangs = map[entity.Language]string{
	entity.LangRU: "russian",
	entity.LangEN: "english",
}

// Cloudflare translates through Workers AI.
type Cloudflare struct {
	url      string
	apiToken string
	client   *http.Client

	backoff time.Duration
}

func NewCloudflare(accountID, apiToken string) *Cloudflare {
	return &Cloudflare{
		url:      fmt.Sprintf(cloudflareEndpoint, accountID, cloudflareModel),
		apiToken: apiToken,
		client:   &http.Client{Timeout: cloudflareTimeout},
		backoff:  time.Second,
	}
}

type cloudflareRequest struct {
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

type cloudflareResponse struct {
	Success bool `json:"success"`
	Result  struct {
		TranslatedText string `json:"translated_text"`
	} `json:"result"`
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Cloudflare) Translate(ctx context.Context, text string, sourceLang, targetLang entity.Language) (string, error) {
	source, ok := cloudflareLangs[sourceLang]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedLanguage, sourceLang)
	}

	target, ok := cloudflareLangs[targetLang]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedLanguage, targetLang)
	}

	payload, err := json.Marshal(cloudflareRequest{Text: text, SourceLang: source, TargetLang: target})
	if err != nil {
		return "", fmt.Errorf("translate: encode request: %w", err)
	}

	body, err := c.send(ctx, payload)
	if err != nil {
		return "", fmt.Errorf("translate: %s to %s: %w", source, target, err)
	}

	var resp cloudflareResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("translate: decode response: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("translate: %s", errorText(resp))
	}

	if resp.Result.TranslatedText == "" {
		return "", fmt.Errorf("translate: %s to %s: empty translation", source, target)
	}

	return resp.Result.TranslatedText, nil
}

// send posts one request, retrying what looks temporary.
// Anything below 500 is returned right away without retrying.
func (c *Cloudflare) send(ctx context.Context, payload []byte) ([]byte, error) {
	var lastErr error

	for attempt := range cloudflareAttempts {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(1<<attempt) * c.backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+c.apiToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		switch {
		case readErr != nil:
			lastErr = readErr
		case resp.StatusCode == http.StatusOK:
			return body, nil
		case resp.StatusCode < http.StatusInternalServerError:
			return nil, fmt.Errorf("status %d: %s", resp.StatusCode, statusText(body))
		default:
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		}
	}

	return nil, fmt.Errorf("after %d attempts: %w", cloudflareAttempts, lastErr)
}

// errorText joins whatever Workers AI said went wrong.
func errorText(resp cloudflareResponse) string {
	if len(resp.Errors) == 0 {
		return "request rejected, no reason given"
	}

	reasons := make([]string, 0, len(resp.Errors))
	for _, e := range resp.Errors {
		reasons = append(reasons, fmt.Sprintf("%d %s", e.Code, e.Message))
	}

	return strings.Join(reasons, "; ")
}

// statusText pulls the reasons out of an error response, falling back to the
// raw body when it is not shaped like one.
func statusText(body []byte) string {
	var resp cloudflareResponse
	if err := json.Unmarshal(body, &resp); err == nil && len(resp.Errors) > 0 {
		return errorText(resp)
	}

	return strings.TrimSpace(string(body))
}
