package agent

import (
	"context"
	"sync"
	"time"
)

// managedSession encapsula una sesión del LLM con su timestamp de último uso.
type managedSession struct {
	Session  Session
	LastUsed time.Time
}

// SessionManager maneja el ciclo de vida de las sesiones multi-turno en memoria.
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

	if exists {
		// Verificar expiración (TTL)
		if now.Sub(ms.LastUsed) < sm.ttl {
			sm.mu.Lock()
			ms.LastUsed = now // Actualizar timestamp de actividad
			sm.mu.Unlock()
			return ms.Session, nil
		}
	}

	// Crear nueva sesión subyacente (LLM)
	newSession, err := llm.NewSession(ctx, systemPrompt, tools)
	if err != nil {
		return nil, err
	}

	// DECORADOR: Envolvemos la sesión nativa con nuestra capa de tolerancia a fallos
	newSession = WrapSession(newSession, sm.debug)

	sm.mu.Lock()
	sm.sessions[userID] = &managedSession{
		Session:  newSession,
		LastUsed: now,
	}
	sm.mu.Unlock()

	return newSession, nil
}
