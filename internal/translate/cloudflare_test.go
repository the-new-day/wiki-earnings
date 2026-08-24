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

const okBody = `{"success":true,"result":{"translated_text":"Fix the tanks page"}}`

// reply is one answer from Workers AI. A test lists one per attempt it expects.
type reply struct {
	status int
	body   string
}

// exchange is what the stubbed service saw and how many times it was called.
type exchange struct {
	attempts int
	auth     string
	request  cloudflareRequest
}

// stub answers with replies in turn, repeating the last one once they run out.
func stub(t *testing.T, replies []reply) (*Cloudflare, *exchange) {
	t.Helper()

	seen := &exchange{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.auth = r.Header.Get("Authorization")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &seen.request))

		answer := replies[min(seen.attempts, len(replies)-1)]
		seen.attempts++

		if answer.status != 0 && answer.status != http.StatusOK {
			w.WriteHeader(answer.status)
		}

		io.WriteString(w, answer.body)
	}))
	t.Cleanup(server.Close)

	return &Cloudflare{url: server.URL, apiToken: "token", client: server.Client(), backoff: 0}, seen
}

func TestCloudflare_Translate(t *testing.T) {
	tests := []struct {
		name            string
		targetLang      entity.Language
		replies         []reply
		wantText        string
		wantErr         error
		wantErrMentions []string
		wantAttempts    int
	}{
		{
			name:         "translates",
			targetLang:   entity.LangEN,
			replies:      []reply{{body: okBody}},
			wantText:     "Fix the tanks page",
			wantAttempts: 1,
		},
		{
			name:       "retries server errors",
			targetLang: entity.LangEN,
			replies: []reply{
				{status: http.StatusBadGateway},
				{status: http.StatusBadGateway},
				{body: okBody},
			},
			wantText:     "Fix the tanks page",
			wantAttempts: 3,
		},
		{
			name:            "gives up once the attempts run out",
			targetLang:      entity.LangEN,
			replies:         []reply{{status: http.StatusBadGateway}},
			wantErrMentions: []string{"502"},
			wantAttempts:    cloudflareAttempts,
		},
		{
			// A rejected token or a spent daily allowance will not come right
			// in a few seconds.
			name:       "does not retry client errors",
			targetLang: entity.LangEN,
			replies: []reply{{
				status: http.StatusTooManyRequests,
				body:   `{"success":false,"errors":[{"code":10000,"message":"out of neurons"}]}`,
			}},
			wantErrMentions: []string{"out of neurons"},
			wantAttempts:    1,
		},
		{
			name:       "reports a request the service rejected",
			targetLang: entity.LangEN,
			replies: []reply{{
				body: `{"success":false,"errors":[{"code":7003,"message":"no route for that URI"}]}`,
			}},
			wantErrMentions: []string{"no route for that URI"},
			wantAttempts:    1,
		},
		{
			name:            "refuses an empty translation",
			targetLang:      entity.LangEN,
			replies:         []reply{{body: `{"success":true,"result":{"translated_text":""}}`}},
			wantErrMentions: []string{"empty translation"},
			wantAttempts:    1,
		},
		{
			name:         "refuses a language the model does not speak",
			targetLang:   entity.Language(99),
			replies:      []reply{{body: okBody}},
			wantErr:      ErrUnsupportedLanguage,
			wantAttempts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, seen := stub(t, tt.replies)

			got, err := client.Translate(context.Background(), "Починить страницу", entity.LangRU, tt.targetLang)

			assert.Equal(t, tt.wantAttempts, seen.attempts)

			if tt.wantErr != nil || len(tt.wantErrMentions) > 0 {
				require.Error(t, err)
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr)
				}
				for _, mention := range tt.wantErrMentions {
					assert.ErrorContains(t, err, mention)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantText, got)
			assert.Equal(t, "Bearer token", seen.auth)
			assert.Equal(t, cloudflareRequest{
				Text:       "Починить страницу",
				SourceLang: "russian",
				TargetLang: "english",
			}, seen.request)
		})
	}
}
