package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"blue/internal/loyverse"
)

// Agent es el cerebro del chatbot Lumi. Conecta un LLM con Loyverse
// usando function calling para responder consultas en lenguaje natural.
type Agent struct {
	llm            LLM
	loyverse       loyverse.Reader
	suppliers      map[string][]string
	debug          bool
	sessionManager *SessionManager
}

// Option configura el Agent.
type Option func(*Agent)

// WithDebug activa el modo debug que logea todo el flujo interno.
func WithDebug(enabled bool) Option {
	return func(a *Agent) { a.debug = enabled }
}

// New crea un nuevo Agent listo para chatear.
func New(llm LLM, loy loyverse.Reader, suppliers map[string][]string, opts ...Option) *Agent {
	a := &Agent{
		llm:       llm,
		loyverse:  loy,
		suppliers: suppliers,
	}
	for _, opt := range opts {
		opt(a)
	}
	a.sessionManager = NewSessionManager(30*time.Minute, a.debug)
	return a
}

func (a *Agent) debugLog(format string, args ...any) {
	if a.debug {
		log.Printf("[DEBUG agent] "+format, args...)
	}
}

// TranscribeAudio utiliza el LLM subyacente para pasar una nota de voz a texto.
func (a *Agent) TranscribeAudio(ctx context.Context, audioData []byte) (string, error) {
	return a.llm.Transcribe(ctx, audioData)
}

// Chat envía un mensaje al modelo, ejecuta el loop de function calling,
// y retorna la respuesta de texto final.
func (a *Agent) Chat(ctx context.Context, userID, message string) (string, error) {
	a.debugLog(">>> mensaje usuario (%s): %q", userID, message)

	session, err := a.sessionManager.GetOrCreate(ctx, userID, a.llm, buildSystemPrompt(), lumiTools())
	if err != nil {
		return "", fmt.Errorf("obteniendo sesión: %w", err)
	}
	a.debugLog("sesión LLM lista para %s", userID)

	text, calls, err := session.Send(ctx, message)
	if err != nil {
		return "", fmt.Errorf("sending message: %w", err)
	}
	a.debugLog("respuesta inicial: text=%q, calls=%d", truncate(text, 100), len(calls))

	for i := 0; i < 5 && len(calls) > 0; i++ {
		a.debugLog("--- iteración %d: %d function call(s) ---", i, len(calls))

		results := make([]ToolResult, len(calls))
		for j, fc := range calls {
			a.debugLog("TOOL CALL: %s(%s)", fc.Name, mustJSONString(fc.Args))

			result, execErr := a.ExecuteTool(ctx, fc.Name, fc.Args)
			if execErr != nil {
				a.debugLog("TOOL ERROR: %s → %v", fc.Name, execErr)
				result = map[string]any{"error": execErr.Error()}
			} else {
				a.debugLog("TOOL RESULT: %s → %s", fc.Name, truncate(mustJSONString(result), 500))
			}
			results[j] = ToolResult{Name: fc.Name, Result: result}
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
