package llm

import (
	"context"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

// mockSession simula el comportamiento de Groq/OpenAI para tests de retry.
type mockSession struct {
	attempts int
}

func (m *mockSession) Send(_ context.Context, _ string) (string, []ToolCall, error) {
	m.attempts++
	if m.attempts < 3 {
		return "", nil, &openai.APIError{
			HTTPStatusCode: 429,
			Message:        "Rate limit exceeded (simulado)",
		}
	}
	return "respuesta exitosa", nil, nil
}

func (m *mockSession) SendToolResults(_ context.Context, _ []ToolResult) (string, []ToolCall, error) {
	return "", nil, nil
}

func TestRetryDecorator_SuccessOnThirdAttempt(t *testing.T) {
	mock := &mockSession{}
	wrapped := WrapSession(mock, true)

	start := time.Now()
	text, _, err := wrapped.Send(context.Background(), "test message")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("se esperaba éxito al final, pero falló: %v", err)
	}
	if text != "respuesta exitosa" {
		t.Fatalf("respuesta inesperada: %s", text)
	}
	if mock.attempts != 3 {
		t.Fatalf("se esperaban 3 intentos, pero se registraron %d", mock.attempts)
	}
	// backoff: 1s (intento 1→2) + 2s (intento 2→3) = ~3s total
	if elapsed < 3*time.Second {
		t.Fatalf("el backoff no está pausando correctamente. Tiempo: %v", elapsed)
	}
	t.Logf("completado en %v con %d intentos", elapsed, mock.attempts)
}
