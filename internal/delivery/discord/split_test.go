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
		fmt.Fprintf(&body, "* [2026-08-20|%d] ((NA)): [Main Page](<https://wiki.pro-tanki.online/en/Main_Page>)\n  10310 💎\n", i)
	}

	return body.String()
}

func TestSplitMessage_ShortContentStaysWhole(t *testing.T) {
	content := editsReport(3)

	assert.Equal(t, []string{content}, splitMessage(content, maxMessageLength))
}

func TestSplitMessage_EveryChunkFitsTheLimit(t *testing.T) {
	chunks := splitMessage(editsReport(200), maxMessageLength)

	require.Greater(t, len(chunks), 1)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), maxMessageLength)
	}
}

func TestSplitMessage_LosesNothing(t *testing.T) {
	content := editsReport(200)

	assert.Equal(t, content, strings.Join(splitMessage(content, maxMessageLength), ""))
}

func TestSplitMessage_BreaksOnLineBoundaries(t *testing.T) {
	for _, chunk := range splitMessage(editsReport(200), maxMessageLength) {
		assert.True(t, strings.HasPrefix(chunk, "* ["), "chunk starts mid-row: %.40q", chunk)
		assert.True(t, strings.HasSuffix(chunk, "💎\n"), "chunk ends mid-row: %.40q", chunk)
	}
}

func TestSplitMessage_KeepsCodeFencesBalanced(t *testing.T) {
	var body strings.Builder
	body.WriteString("```\n")
	for i := range 200 {
		fmt.Fprintf(&body, "/givecry Editor%d %d\n", i, i*100)
	}
	body.WriteString("```")

	chunks := splitMessage(body.String(), maxMessageLength)

	require.Greater(t, len(chunks), 1)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), maxMessageLength)
		assert.Zero(t, strings.Count(chunk, "```")%2, "unbalanced fence in %.40q", chunk)
	}
}

func TestSplitMessage_CutsOverlongLineOnRuneBoundaries(t *testing.T) {
	content := strings.Repeat("💎", maxMessageLength)

	chunks := splitMessage(content, maxMessageLength)

	require.Greater(t, len(chunks), 1)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), maxMessageLength)
		assert.True(t, utf8Valid(chunk), "chunk cut mid-rune")
	}
	assert.Equal(t, content, strings.Join(chunks, ""))
}

func utf8Valid(s string) bool {
	return !strings.ContainsRune(s, '�') && strings.ToValidUTF8(s, "�") == s
}
