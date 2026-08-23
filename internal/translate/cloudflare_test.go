package translate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

func stub(t *testing.T, handler http.HandlerFunc) *Cloudflare {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &Cloudflare{
		url:      server.URL,
		apiToken: "token",
		client:   server.Client(),
		backoff:  0,
	}
}

func TestCloudflare_SendsTheRequestWorkersAIExpects(t *testing.T) {
	var got cloudflareRequest
	var auth string

	client := stub(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &got))

		io.WriteString(w, `{"success":true,"result":{"translated_text":"Fix the tanks page"}}`)
	})

	out, err := client.Translate(context.Background(), "Починить страницу", entity.LangRU, entity.LangEN)

	require.NoError(t, err)
	assert.Equal(t, "Fix the tanks page", out)
	assert.Equal(t, "Bearer token", auth)
	assert.Equal(t, "Починить страницу", got.Text)

	assert.Equal(t, "russian", got.SourceLang)
	assert.Equal(t, "english", got.TargetLang)
}

func TestCloudflare_RetriesServerErrors(t *testing.T) {
	attempts := 0

	client := stub(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		io.WriteString(w, `{"success":true,"result":{"translated_text":"ok"}}`)
	})

	out, err := client.Translate(context.Background(), "текст", entity.LangRU, entity.LangEN)

	require.NoError(t, err)
	assert.Equal(t, "ok", out)
	assert.Equal(t, 3, attempts)
}

func TestCloudflare_DoesNotRetryClientErrors(t *testing.T) {
	attempts := 0

	client := stub(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"success":false,"errors":[{"code":10000,"message":"out of neurons"}]}`)
	})

	_, err := client.Translate(context.Background(), "текст", entity.LangRU, entity.LangEN)

	require.Error(t, err)
	assert.Equal(t, 1, attempts)
	assert.Contains(t, err.Error(), "out of neurons")
}

func TestCloudflare_ReportsRejectedRequests(t *testing.T) {
	client := stub(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"success":false,"errors":[{"code":7003,"message":"no route for that URI"}]}`)
	})

	_, err := client.Translate(context.Background(), "текст", entity.LangRU, entity.LangEN)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no route for that URI")
}

func TestCloudflare_RejectsEmptyTranslation(t *testing.T) {
	client := stub(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"success":true,"result":{"translated_text":""}}`)
	})

	_, err := client.Translate(context.Background(), "текст", entity.LangRU, entity.LangEN)

	assert.ErrorContains(t, err, "empty translation")
}

func TestCloudflare_RejectsUnknownLanguage(t *testing.T) {
	client := stub(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not have reached the service")
	})

	_, err := client.Translate(context.Background(), "текст", entity.LangRU, entity.Language(99))

	assert.ErrorIs(t, err, ErrUnsupportedLanguage)
}
