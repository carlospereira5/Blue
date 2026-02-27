package agent

import (
	"context"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

// mockSession simula el comportamiento de Groq/OpenAI.
type mockSession struct {
	attempts int
}

func (m *mockSession) Send(ctx context.Context, message string) (string, []ToolCall, error) {
	m.attempts++
	
	// Simulamos que la API rechaza las 2 primeras peticiones por Rate Limit
	if m.attempts < 3 {
		return "", nil, &openai.APIError{
			HTTPStatusCode: 429, 
			Message:        "Rate limit exceeded (Simulado)",
		}
	}
	
	// A la tercera petición, la API responde bien
	return "respuesta exitosa", nil, nil
}

func (m *mockSession) SendToolResults(ctx context.Context, results []ToolResult) (string, []ToolCall, error) {
	return "", nil, nil
}

func TestRetryDecorator_SuccessOnThirdAttempt(t *testing.T) {
	mock := &mockSession{}
	
	// Envolvemos el mock con nuestro decorador (debug=true para ver los logs en el test)
	wrappedSession := WrapSession(mock, true)

	start := time.Now()
	
	// Ejecutamos la petición
	text, _, err := wrappedSession.Send(context.Background(), "test message")

	elapsed := time.Since(start)

	// Verificaciones
	if err != nil {
		t.Fatalf("se esperaba éxito al final, pero falló: %v", err)
	}

	if text != "respuesta exitosa" {
		t.Fatalf("respuesta inesperada: %s", text)
	}

	if mock.attempts != 3 {
		t.Fatalf("se esperaban 3 intentos en el mock, pero se registraron %d", mock.attempts)
	}

	// El backoff es 1s (intento 1 -> 2) + 2s (intento 2 -> 3) = ~3 segundos de pausa total
	if elapsed < 3*time.Second {
		t.Fatalf("el backoff no está pausando la goroutine correctamente. Tiempo transcurrido: %v", elapsed)
	}
	
	t.Logf("Test completado con éxito. Tiempo total: %v. Intentos: %d", elapsed, mock.attempts)
}
