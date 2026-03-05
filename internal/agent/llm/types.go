package llm

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
	Type        string // "string", "integer", "number", "boolean"
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
