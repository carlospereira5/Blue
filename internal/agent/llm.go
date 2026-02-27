package agent

import "context"

// ToolDef define una herramienta que el LLM puede invocar.
type ToolDef struct {
	Name        string
	Description string
	Parameters  []ParamDef
	Required    []string
}

// ParamDef define un parámetro de una herramienta.
type ParamDef struct {
	Name        string
	Type        string // "string", "integer"
	Description string
	Enum        []string
}

// ToolCall representa una invocación de herramienta pedida por el LLM.
type ToolCall struct {
	Name string
	Args map[string]any
}

// ToolResult es el resultado de ejecutar una herramienta.
type ToolResult struct {
	Name   string
	Result map[string]any
}

// Session es una conversación stateful con un LLM.
type Session interface {
	Send(ctx context.Context, message string) (text string, calls []ToolCall, err error)
	SendToolResults(ctx context.Context, results []ToolResult) (text string, calls []ToolCall, err error)
}

// LLM crea sesiones de conversación y transcribe audio.
type LLM interface {
	NewSession(ctx context.Context, systemPrompt string, tools []ToolDef) (Session, error)
	// Transcribe convierte datos de audio crudo en texto.
	Transcribe(ctx context.Context, audioData []byte) (string, error)
}
