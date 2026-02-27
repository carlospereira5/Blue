package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/sashabaranov/go-openai"
)

// retrySession envuelve una Session y añade lógica de reintento exponencial.
// Esto aísla el manejo de fallos transitorios de la lógica de negocio.
type retrySession struct {
	inner      Session
	maxRetries int
	debug      bool
}

// WrapSession crea un decorador para reintentar errores de red o rate limits.
func WrapSession(inner Session, debug bool) Session {
	return &retrySession{
		inner:      inner,
		maxRetries: 3, // 3 intentos totales (1s, 2s, 4s backoff)
		debug:      debug,
	}
}

func (s *retrySession) Send(ctx context.Context, message string) (string, []ToolCall, error) {
	return s.withRetry(ctx, "Send", func() (string, []ToolCall, error) {
		return s.inner.Send(ctx, message)
	})
}

func (s *retrySession) SendToolResults(ctx context.Context, results []ToolResult) (string, []ToolCall, error) {
	return s.withRetry(ctx, "SendToolResults", func() (string, []ToolCall, error) {
		return s.inner.SendToolResults(ctx, results)
	})
}

// withRetry ejecuta la operación bloqueante controlando el scheduler de Go vía select.
func (s *retrySession) withRetry(ctx context.Context, opName string, op func() (string, []ToolCall, error)) (string, []ToolCall, error) {
	var (
		text  string
		calls []ToolCall
		err   error
	)

	backoff := 1 * time.Second

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		text, calls, err = op()
		if err == nil {
			return text, calls, nil // Éxito, salida temprana
		}

		if !s.isRetryable(err) {
			return text, calls, err // Error fatal (ej. 400, 401), no reintentar
		}

		if attempt == s.maxRetries {
			break // Se acabaron los intentos
		}

		if s.debug {
			log.Printf("[DEBUG agent] ⚠ %s fallo transitorio (intento %d/%d): %v. Reintentando en %v...", opName, attempt, s.maxRetries, err, backoff)
		}

		// Suspender goroutine liberando CPU (Zero-CPU idle)
		select {
		case <-ctx.Done():
			return text, calls, ctx.Err() // Contexto cancelado por el usuario/timeout global
		case <-time.After(backoff):
			backoff *= 2 // Retroceso exponencial (1s -> 2s -> 4s)
		}
	}

	return text, calls, fmt.Errorf("fallo final tras %d intentos: %w", s.maxRetries, err)
}

// isRetryable inspecciona la memoria del error para inferir si vale la pena reintentar.
func (s *retrySession) isRetryable(err error) bool {
	// 1. Errores de API (Específico para proveedor OpenAI/Groq)
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.HTTPStatusCode
		// 429 = Rate Limit / Too Many Requests
		// >= 500 = Fallo interno del clúster de LPU (500, 502, 503)
		if code == 429 || code >= 500 {
			return true
		}
		return false
	}

	// 2. Errores de red a nivel socket de SO (ej. connection reset by peer, timeouts)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}
