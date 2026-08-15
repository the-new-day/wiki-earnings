package metric

import (
	"fmt"
	"math"
)

type Section struct {
	Level int
	Line  string
}

type ArticleState struct {
	Words      []string
	Categories []string
	Links      []string
	Images     []string
	Sections   []Section
	Templates  []string
}

type baseMetric struct {
	weight int
}

func (m *baseMetric) Weight() int {
	return m.weight
}

// density returns density of items compared to insideOf in percent.
func density(item, insideOf int) float64 {
	return float64(item) / float64(insideOf) * 100
}

// sat returns min(x/cap, 1.0).
// If cap <= 0, panics; if x < 0, returns 0.
func sat(x, cap float64) float64 {
	if cap <= 0 {
		panic(fmt.Sprintf("sat(%f, %f): cap is <= 0", x, cap))
	}
	return max(0.0, min(x/cap, 1.0))
}

// satInt returns min(x/cap, 1.0).
// If cap <= 0, panics; if x < 0, returns 0.
func satInt(x, cap int) float64 {
	if cap <= 0 {
		panic(fmt.Sprintf("satInt(%d, %d): cap is <= 0", x, cap))
	}
	return max(0, min(float64(x)/float64(cap), 1.0))
}

// inNhood checks if x is in the neighborhood of base, i.e. is in range [base-eps; base+eps].
func inNhood(x, base, eps int) bool {
	return math.Abs(float64(x-base)) <= float64(eps)
}
