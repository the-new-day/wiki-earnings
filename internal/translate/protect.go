package translate

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

type Translator interface {
	Translate(ctx context.Context, text string, sourceLang, targetLang entity.Language) (string, error)
}

var protectedSpan = regexp.MustCompile(strings.Join([]string{
	"```(?s:.*?)```", // fenced code block
	"`[^`\n]+`",      // inline code
	"\\[[^\\]]*\\]\\(\\s*<?[^)\\s]*>?\\s*\\)",             // markdown link, label and all
	"<a?:[0-9A-Za-z_]+:[0-9]+>",                           // custom emoji
	"</[0-9A-Za-z_ -]+:[0-9]+>",                           // slash command mention
	"<@[!&]?[0-9]+>",                                      // user or role mention
	"<#[0-9]+>",                                           // channel mention
	"<t:[0-9]+(?::[tTdDfFR])?>",                           // timestamp
	"<https?://[^>\\s]+>",                                 // link with its embed suppressed
	"https?://[^\\s<>()\\[\\]]*[^\\s<>()\\[\\].,;:!?'\"]", // bare link
	"@everyone",
	"@here",
}, "|"))

// placeholder matches what a protected span was replaced with, allowing the
// spaces a model tends to put inside braces.
var placeholder = regexp.MustCompile(`\{\s*([0-9]+)\s*\}`)

// Protected translates through inner with everything a model would corrupt
// replaced by {0}, {1} and so on, put back afterwards.
type Protected struct {
	inner Translator
}

func Protect(inner Translator) *Protected {
	return &Protected{inner: inner}
}

// Translate masks the protected spans, translates what is left, and puts them
// back. When the placeholders do not come back intact, it falls back to
// translating the text around the spans instead.
func (p *Protected) Translate(ctx context.Context, text string, sourceLang, targetLang entity.Language) (string, error) {
	parts := split(text)

	if !anyTranslatable(parts) {
		return text, nil
	}

	masked, spans := mask(parts)
	if len(spans) == 0 {
		return p.inner.Translate(ctx, text, sourceLang, targetLang)
	}

	translated, err := p.inner.Translate(ctx, masked, sourceLang, targetLang)
	if err != nil {
		return "", err
	}

	restored, err := restore(translated, spans)
	if err == nil {
		return restored, nil
	}

	log.Printf("translate: %v; sent %q, got %q; translating around the protected spans instead",
		err, masked, translated)

	return p.aroundSpans(ctx, parts, sourceLang, targetLang)
}

// aroundSpans translates each run of text between the protected spans on its
// own. It costs a request per run and reads worse for it, the model never
// seeing the sentence whole.
func (p *Protected) aroundSpans(
	ctx context.Context,
	parts []part,
	sourceLang, targetLang entity.Language,
) (string, error) {
	var out strings.Builder

	for _, part := range parts {
		if part.protected || !hasLetters(part.text) {
			out.WriteString(part.text)
			continue
		}

		lead, body, tail := edges(part.text)

		translated, err := p.inner.Translate(ctx, body, sourceLang, targetLang)
		if err != nil {
			return "", err
		}

		out.WriteString(lead)
		out.WriteString(translated)
		out.WriteString(tail)
	}

	return out.String(), nil
}

// part is a run of the text, either protected or up for translation.
type part struct {
	text      string
	protected bool
}

func split(text string) []part {
	var parts []part

	end := 0
	for _, span := range protectedSpan.FindAllStringIndex(text, -1) {
		if span[0] > end {
			parts = append(parts, part{text: text[end:span[0]]})
		}

		parts = append(parts, part{text: text[span[0]:span[1]], protected: true})
		end = span[1]
	}

	if end < len(text) {
		parts = append(parts, part{text: text[end:]})
	}

	return parts
}

// mask swaps every protected part for its placeholder, returning the spans in
// the order the placeholders number them.
func mask(parts []part) (masked string, spans []string) {
	var out strings.Builder

	for _, part := range parts {
		if !part.protected {
			out.WriteString(part.text)
			continue
		}

		fmt.Fprintf(&out, "{%d}", len(spans))
		spans = append(spans, part.text)
	}

	return out.String(), spans
}

// restore puts the spans back where their placeholders ended up. A placeholder
// the model dropped, duplicated or invented is refused.
func restore(translated string, spans []string) (string, error) {
	found := make([]int, len(spans))

	var faults []string

	out := placeholder.ReplaceAllStringFunc(translated, func(match string) string {
		index, err := strconv.Atoi(placeholder.FindStringSubmatch(match)[1])
		if err != nil || index >= len(spans) {
			faults = append(faults, match+" was never sent")
			return match
		}

		found[index]++

		return spans[index]
	})

	for index, times := range found {
		if times != 1 {
			faults = append(faults, fmt.Sprintf("{%d} came back %d times", index, times))
		}
	}

	if len(faults) > 0 {
		return "", fmt.Errorf("of %d placeholders, %s", len(spans), strings.Join(faults, ", "))
	}

	return out, nil
}

func anyTranslatable(parts []part) bool {
	for _, part := range parts {
		if !part.protected && hasLetters(part.text) {
			return true
		}
	}

	return false
}

func hasLetters(text string) bool {
	return strings.IndexFunc(text, unicode.IsLetter) >= 0
}

// edges cuts the surrounding whitespace off a run of text.
func edges(text string) (lead, body, tail string) {
	body = strings.TrimSpace(text)
	lead = text[:strings.Index(text, body)]
	tail = text[len(lead)+len(body):]

	return lead, body, tail
}
