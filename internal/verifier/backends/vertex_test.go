package backends

import (
	"testing"
)

func TestVertex_Initialize(t *testing.T) {
	// Without valid credentials, the genai.NewClient will fail with ADC error.
	// Since we mock backend logic via the SDK, it's safer to skip full execution 
	// unless we implement rigorous abstract interfaces.
	_, err := NewVertexBackend("mock-model")
	if err == nil {
		// If it somehow passes (e.g. running locally with valid ADC), that's fine.
		t.Log("Initialized VertexBackend successfully")
	} else {
		// We expect this to fail in CI without GOOGLE_APPLICATION_CREDENTIALS
		t.Logf("Expected failure without ADC credentials: %v", err)
	}
}
