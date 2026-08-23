package translate

import (
	"context"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

// Passthrough is a Translator that does not translate: it hands the text back
// exactly as it came in.
type Passthrough struct{}

func NewPassthrough() *Passthrough {
	return &Passthrough{}
}

func (*Passthrough) Translate(_ context.Context, text string, _, _ entity.Language) (string, error) {
	return text, nil
}
