package matcher

import (
	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
)

// Config holds matcher configuration
type Config struct {
	AmountTolerance float64 // Default: 0.01 (1 cent)
	DateTolerance   int     // Days tolerance (default: 5)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		AmountTolerance: 0.01,
		DateTolerance:   5,
	}
}

// MatchResult contains match information
type MatchResult struct {
	Transaction *monarch.Transaction
	DateDiff    float64 // Days difference
	AmountDiff  float64 // Absolute amount difference
	Confidence  float64 // 0-1 score (for future use)
}
