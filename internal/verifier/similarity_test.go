package verifier

import (
	"math"
	"testing"
)

func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestCosineDistance(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		expected float64
	}{
		{
			name:     "Identical Vectors",
			a:        []float64{1.0, 2.0, 3.0},
			b:        []float64{1.0, 2.0, 3.0},
			expected: 1.0,
		},
		{
			name:     "Orthogonal Vectors",
			a:        []float64{1.0, 0.0},
			b:        []float64{0.0, 1.0},
			expected: 0.0,
		},
		{
			name:     "Opposite Vectors",
			a:        []float64{1.0, 1.0},
			b:        []float64{-1.0, -1.0},
			expected: -1.0,
		},
		{
			name:     "Different Length",
			a:        []float64{1.0, 2.0},
			b:        []float64{1.0},
			expected: 0.0,
		},
		{
			name:     "Zero Vectors",
			a:        []float64{0.0, 0.0},
			b:        []float64{0.0, 0.0},
			expected: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CosineDistance(tc.a, tc.b)
			if !floatEquals(result, tc.expected) {
				t.Errorf("expected %f, got %f", tc.expected, result)
			}
		})
	}
}
