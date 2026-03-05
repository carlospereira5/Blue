package llm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// ── SessionManager ────────────────────────────────────────────────────────────

// managedSession encapsula una sesión LLM con su timestamp de último uso.
type managedSession struct {
	Session  Session
	LastUsed time.Time
}

// SessionManager gestiona el ciclo de vida de sesiones multi-turno en memoria.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*managedSession
	ttl      time.Duration
	debug    bool
}

// NewSessionManager crea un manager con el TTL especificado.
func NewSessionManager(ttl time.Duration, debug bool) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*managedSession),
		ttl:      ttl,
		debug:    debug,
	}
}

// GetOrCreate recupera una sesión existente o crea una nueva si no existe o expiró.
func (sm *SessionManager) GetOrCreate(ctx context.Context, userID string, llm LLM, systemPrompt string, tools []ToolDef) (Session, error) {
	sm.mu.RLock()
	ms, exists := sm.sessions[userID]
	sm.mu.RUnlock()

	now := time.Now()

	if exists && now.Sub(ms.LastUsed) < sm.ttl {
		sm.mu.Lock()
		ms.LastUsed = now
		sm.mu.Unlock()
		return ms.Session, nil
	}

	newSession, err := llm.NewSession(ctx, systemPrompt, tools)
	if err != nil {
		return nil, err
	}

	// Envolvemos la sesión con tolerancia a fallos.
	newSession = WrapSession(newSession, sm.debug)

	sm.mu.Lock()
	sm.sessions[userID] = &managedSession{Session: newSession, LastUsed: now}
	sm.mu.Unlock()

	return newSession, nil
}

// ── retrySession ─────────────────────────────────────────────────────────────

// retrySession envuelve una Session con lógica de reintento exponencial.
type retrySession struct {
	inner      Session
	maxRetries int
	debug      bool
}

// WrapSession crea un decorador para reintentar errores de red o rate limits.
func WrapSession(inner Session, debug bool) Session {
	return &retrySession{inner: inner, maxRetries: 3, debug: debug}
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
			return text, calls, nil
		}
		if !s.isRetryable(err) {
			return text, calls, err
		}
		if attempt == s.maxRetries {
			break
		}
		if s.debug {
			log.Printf("[llm] retry %s (intento %d/%d, próximo en %v): %v", opName, attempt, s.maxRetries, backoff, err)
		}
		select {
		case <-ctx.Done():
			return text, calls, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	return text, calls, fmt.Errorf("fallo final tras %d intentos: %w", s.maxRetries, err)
}

func (s *retrySession) isRetryable(err error) bool {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.HTTPStatusCode
		return code == 429 || code >= 500
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
