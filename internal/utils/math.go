package utils

import (
	"fmt"
	"math"

	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
)

// Density returns density of items compared to insideOf in percent.
func Density(item, insideOf int) float64 {
	return float64(item) / float64(insideOf) * 100
}

// Sat returns min(x/cap, 1.0).
// If cap <= 0, panics; if x < 0, returns 0.
func Sat(x, cap float64) float64 {
	if cap <= 0 {
		panic(fmt.Sprintf("sat(%f, %f): cap is <= 0", x, cap))
	}
	return max(0.0, min(x/cap, 1.0))
}

// SatInt returns min(x/cap, 1.0).
// If cap <= 0, panics; if x < 0, returns 0.
func SatInt(x, cap int) float64 {
	if cap <= 0 {
		panic(fmt.Sprintf("satInt(%d, %d): cap is <= 0", x, cap))
	}
	return max(0, min(float64(x)/float64(cap), 1.0))
}

func Logistic(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-analyze.WordCountSteepness*(x-analyze.WordCountMidpoint)))
}
