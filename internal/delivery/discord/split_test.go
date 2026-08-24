package discord

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func editsReport(rows int) string {
	var body strings.Builder

	for i := range rows {
		fmt.Fprintf(&body, "* [2026-08-20|%d] ((NA)): [Main Page](https://wiki.pro-tanki.online/en/Main_Page)\n  10310 💎\n", i)
	}

	return body.String()
}

func fencedCommands(rows int) string {
	var body strings.Builder

	body.WriteString("```\n")
	for i := range rows {
		fmt.Fprintf(&body, "/givecry Editor%d %d\n", i, i*100)
	}
	body.WriteString("```")

	return body.String()
}

func breaksOnLineBoundaries(t *testing.T, chunks []string) {
	for _, chunk := range chunks {
		assert.True(t, strings.HasPrefix(chunk, "* ["), "chunk starts mid-row: %.40q", chunk)
		assert.True(t, strings.HasSuffix(chunk, "💎\n"), "chunk ends mid-row: %.40q", chunk)
	}
}

func keepsFencesBalanced(t *testing.T, chunks []string) {
	for _, chunk := range chunks {
		assert.Zero(t, strings.Count(chunk, "```")%2, "unbalanced fence in %.40q", chunk)
	}
}

func cutsOnRuneBoundaries(t *testing.T, chunks []string) {
	for _, chunk := range chunks {
		assert.True(t, validUTF8(chunk), "chunk cut mid-rune")
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name string
		// content is what gets split; every chunk of it has to fit the limit.
		content string
		// wantSplit says whether the content is expected to need more than one
		// message.
		wantSplit bool
		// wantWhole says whether the chunks put back together are the content
		// again. A reopened code fence adds characters that were not there.
		wantWhole bool
		assert    func(*testing.T, []string)
	}{
		{
			name:      "short content stays whole",
			content:   editsReport(3),
			wantSplit: false,
			wantWhole: true,
		},
		{
			name:      "a long report is cut between rows",
			content:   editsReport(200),
			wantSplit: true,
			wantWhole: true,
			assert:    breaksOnLineBoundaries,
		},
		{
			name:      "a cut code block is closed and reopened",
			content:   fencedCommands(200),
			wantSplit: true,
			assert:    keepsFencesBalanced,
		},
		{
			name:      "a single overlong line is cut between runes",
			content:   strings.Repeat("💎", maxMessageLength),
			wantSplit: true,
			wantWhole: true,
			assert:    cutsOnRuneBoundaries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := splitMessage(tt.content, maxMessageLength)

			if tt.wantSplit {
				require.Greater(t, len(chunks), 1)
			} else {
				require.Len(t, chunks, 1)
			}

			for _, chunk := range chunks {
				assert.LessOrEqual(t, len(chunk), maxMessageLength)
			}

			if tt.wantWhole {
				assert.Equal(t, tt.content, strings.Join(chunks, ""))
			}

			if tt.assert != nil {
				tt.assert(t, chunks)
			}
		})
	}
}

func validUTF8(s string) bool {
	return !strings.ContainsRune(s, '�') && strings.ToValidUTF8(s, "�") == s
}
