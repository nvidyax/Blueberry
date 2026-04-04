package verifier

import (
	"math"
)

// CosineDistance calculates the cosine similarity between two vectors a and b.
// Returns a value between -1.0 and 1.0. Higher is more similar.
func CosineDistance(a, b []float64) float64 {
	var dotProduct, normA, normB float64

	if len(a) != len(b) || len(a) == 0 {
		return 0.0 // Mismatched or empty vectors
	}

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
