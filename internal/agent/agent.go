// Package agent es el orquestador central de Aria.
// Conecta el LLM con las herramientas disponibles y gestiona el ciclo de vida
// de las conversaciones. No contiene lógica de negocio ni acceso directo a datos.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	agentllm "aria/internal/agent/llm"
	agenttools "aria/internal/agent/tools"
	"aria/internal/db"
	"aria/internal/loyverse"
)

// Agent orquesta el flujo conversacional de Aria:
// recibe mensajes → gestiona sesión LLM → despacha tool calls → retorna respuesta.
type Agent struct {
	llm            agentllm.LLM
	executor       *agenttools.Executor
	store          db.Store // nil-safe: para cargar perfil/memorias del usuario
	debug          bool
	sessionManager *agentllm.SessionManager
}

// agentConfig acumula las opciones antes de construir el Agent.
type agentConfig struct {
	debug bool
	store db.Store
}

// Option configura el Agent en la construcción.
type Option func(*agentConfig)

// WithDebug activa el modo debug que loguea todo el flujo interno.
func WithDebug(enabled bool) Option {
	return func(c *agentConfig) { c.debug = enabled }
}

// WithStore inyecta la base de datos local. Si está presente, el DataReader
// preferirá la DB sobre Loyverse para todas las lecturas.
func WithStore(store db.Store) Option {
	return func(c *agentConfig) { c.store = store }
}

// New crea un Agent listo para chatear.
// loy puede ser nil solo en tests donde no se llaman tools que lean datos.
func New(l agentllm.LLM, loy loyverse.Reader, suppliers map[string][]string, opts ...Option) *Agent {
	cfg := &agentConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	reader := agenttools.NewFallbackReader(cfg.store, loy)
	reader = agenttools.NewCachingReader(reader, 5*time.Minute)
	executor := agenttools.NewExecutor(reader, cfg.store, suppliers, cfg.debug)

	return &Agent{
		llm:            l,
		executor:       executor,
		store:          cfg.store,
		debug:          cfg.debug,
		sessionManager: agentllm.NewSessionManager(30*time.Minute, cfg.debug),
	}
}

func (a *Agent) debugLog(format string, args ...any) {
	if a.debug {
		log.Printf("[DEBUG agent] "+format, args...)
	}
}

// TranscribeAudio convierte una nota de voz (OGG) a texto usando el LLM.
func (a *Agent) TranscribeAudio(ctx context.Context, audioData []byte) (string, error) {
	return a.llm.Transcribe(ctx, audioData)
}

// ExecuteTool despacha una tool call al executor. Público para testing.
func (a *Agent) ExecuteTool(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	return a.executor.Execute(ctx, name, args)
}

// Chat envía un mensaje al LLM, ejecuta el loop de function calling,
// y retorna la respuesta de texto final.
func (a *Agent) Chat(ctx context.Context, userID, message string) (string, error) {
	a.debugLog(">>> mensaje usuario (%s): %q", userID, message)

	// Inyectamos el userID en el context para que las tools (ej: save_memory) puedan accederlo.
	ctx = agenttools.ContextWithUserID(ctx, userID)

	session, err := a.sessionManager.GetOrCreate(ctx, userID, a.llm, buildSystemPrompt(ctx, a.store, userID), agenttools.AriaTools())
	if err != nil {
		return "", fmt.Errorf("obteniendo sesión: %w", err)
	}

	text, calls, err := session.Send(ctx, message)
	if err != nil {
		return "", fmt.Errorf("sending message: %w", err)
	}
	a.debugLog("respuesta inicial: text=%q, calls=%d", truncate(text, 100), len(calls))

	for i := 0; i < 5 && len(calls) > 0; i++ {
		a.debugLog("--- iteración %d: %d function call(s) ---", i, len(calls))

		results := make([]agentllm.ToolResult, len(calls))
		for j, fc := range calls {
			a.debugLog("TOOL CALL: %s(%s)", fc.Name, mustJSONString(fc.Args))
			result, execErr := a.executor.Execute(ctx, fc.Name, fc.Args)
			if execErr != nil {
				a.debugLog("TOOL ERROR: %s → %v", fc.Name, execErr)
				result = map[string]any{"error": execErr.Error()}
			} else {
				a.debugLog("TOOL RESULT: %s → %s", fc.Name, truncate(mustJSONString(result), 500))
			}
			results[j] = agentllm.ToolResult{Name: fc.Name, Result: result}
		}

		text, calls, err = session.SendToolResults(ctx, results)
		if err != nil {
			return "", fmt.Errorf("sending tool results: %w", err)
		}
		a.debugLog("respuesta tras tool results: text=%q, calls=%d", truncate(text, 100), len(calls))
	}

	a.debugLog("<<< respuesta final: %s", truncate(text, 200))
	if text == "" {
		return "No pude generar una respuesta. Intentá reformular tu pregunta.", nil
	}
	return text, nil
}

func mustJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(b)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
