// Package llm define las interfaces y tipos para comunicarse con modelos de lenguaje.
// Contiene las implementaciones concretas (Gemini, OpenAI/Groq) y la gestión
// del ciclo de vida de sesiones (SessionManager, retrySession).
package llm

import "context"

// LLM crea sesiones de conversación y transcribe audio.
type LLM interface {
	NewSession(ctx context.Context, systemPrompt string, tools []ToolDef) (Session, error)
	Transcribe(ctx context.Context, audioData []byte) (string, error)
}

// Session es una conversación stateful con un LLM.
type Session interface {
	Send(ctx context.Context, message string) (text string, calls []ToolCall, err error)
	SendToolResults(ctx context.Context, results []ToolResult) (text string, calls []ToolCall, err error)
}
