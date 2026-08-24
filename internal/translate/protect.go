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
	"```(?s:.*?)```",                  // fenced code block
	"`[^`\n]+`",                       // inline code
	"\\](\\(\\s*<?[^)\\s]*>?\\s*\\))", // markdown link address
	"<a?:[0-9A-Za-z_]+:[0-9]+>",       // custom emoji
	"</[0-9A-Za-z_ -]+:[0-9]+>",       // slash command mention
	"<@[!&]?[0-9]+>",                  // user or role mention
	"<#[0-9]+>",                       // channel mention
	"<t:[0-9]+(?::[tTdDfFR])?>",       // timestamp
	"<https?://[^>\\s]+>",             // link with its embed suppressed
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

	log.Printf("translate: %v, translating around the protected spans instead", err)

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
	for _, match := range protectedSpan.FindAllStringSubmatchIndex(text, -1) {
		start, stop := protectedRange(match)

		if start > end {
			parts = append(parts, part{text: text[end:start]})
		}

		parts = append(parts, part{text: text[start:stop], protected: true})
		end = stop
	}

	if end < len(text) {
		parts = append(parts, part{text: text[end:]})
	}

	return parts
}

// protectedRange is the part of a match that must not be translated: the
// captured group where an alternative has one, the whole match otherwise.
func protectedRange(match []int) (start, stop int) {
	for i := 2; i+1 < len(match); i += 2 {
		if match[i] >= 0 {
			return match[i], match[i+1]
		}
	}

	return match[0], match[1]
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

	var unknown error

	out := placeholder.ReplaceAllStringFunc(translated, func(match string) string {
		index, err := strconv.Atoi(placeholder.FindStringSubmatch(match)[1])
		if err != nil || index >= len(spans) {
			unknown = fmt.Errorf("%s came back as %s", plural(len(spans)), match)
			return match
		}

		found[index]++

		return spans[index]
	})

	if unknown != nil {
		return "", unknown
	}

	for index, times := range found {
		if times != 1 {
			return "", fmt.Errorf("{%d} came back %d times, not once", index, times)
		}
	}

	return out, nil
}

// plural names how many placeholders were sent,
// for an error about one that came back as something else.
func plural(spans int) string {
	if spans == 1 {
		return "the only placeholder"
	}

	return fmt.Sprintf("one of %d placeholders", spans)
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
