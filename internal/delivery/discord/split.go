package discord

import (
	"strings"
	"unicode/utf8"
)

const (
	maxMessageLength = 2000
	fenceReserve     = 16
)

func splitMessage(content string, limit int) []string {
	if len(content) <= limit {
		return []string{content}
	}

	budget := limit
	if strings.Contains(content, "```") {
		budget -= fenceReserve
	}

	var (
		chunks []string
		cur    strings.Builder
	)

	for _, line := range wrapLongLines(strings.SplitAfter(content, "\n"), budget) {
		if cur.Len() > 0 && cur.Len()+len(line) > budget {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}

		cur.WriteString(line)
	}

	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}

	return reopenFences(chunks)
}

func wrapLongLines(lines []string, limit int) []string {
	wrapped := make([]string, 0, len(lines))

	for _, line := range lines {
		for len(line) > limit {
			cut := limit
			for cut > 0 && !utf8.RuneStart(line[cut]) {
				cut--
			}

			wrapped = append(wrapped, line[:cut])
			line = line[cut:]
		}

		wrapped = append(wrapped, line)
	}

	return wrapped
}

func reopenFences(chunks []string) []string {
	fence := ""

	for idx, chunk := range chunks {
		if fence != "" {
			chunk = fence + "\n" + chunk
		}

		if fence = openFence(chunk); fence != "" {
			chunk = strings.TrimRight(chunk, "\n") + "\n```"
		}

		chunks[idx] = chunk
	}

	return chunks
}

func openFence(chunk string) string {
	fence := ""

	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "```") {
			continue
		}

		if fence == "" {
			fence = line
		} else {
			fence = ""
		}
	}

	return fence
}
