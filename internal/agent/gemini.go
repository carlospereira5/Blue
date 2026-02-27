package agent

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// GeminiLLM implementa LLM usando el SDK de Google Gemini.
type GeminiLLM struct {
	client *genai.Client
	model  string
}

// NewGeminiLLM crea un LLM backed by Gemini.
func NewGeminiLLM(client *genai.Client, model string) *GeminiLLM {
	return &GeminiLLM{client: client, model: model}
}

// NewSession crea una sesión de chat con Gemini.
func (g *GeminiLLM) NewSession(ctx context.Context, systemPrompt string, tools []ToolDef) (Session, error) {
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(systemPrompt)},
			Role:  "user",
		},
		Tools:       toGeminiTools(tools),
		Temperature: genai.Ptr[float32](0.3),
	}

	chat, err := g.client.Chats.Create(ctx, g.model, config, nil)
	if err != nil {
		return nil, fmt.Errorf("creating gemini chat: %w", err)
	}

	return &geminiSession{chat: chat}, nil
}

type geminiSession struct {
	chat *genai.Chat
}

func (s *geminiSession) Send(ctx context.Context, message string) (string, []ToolCall, error) {
	resp, err := s.chat.Send(ctx, genai.NewPartFromText(message))
	if err != nil {
		return "", nil, err
	}
	return parseGeminiResponse(resp)
}

func (s *geminiSession) SendToolResults(ctx context.Context, results []ToolResult) (string, []ToolCall, error) {
	parts := make([]*genai.Part, len(results))
	for i, r := range results {
		parts[i] = genai.NewPartFromFunctionResponse(r.Name, r.Result)
	}

	resp, err := s.chat.Send(ctx, parts...)
	if err != nil {
		return "", nil, err
	}
	return parseGeminiResponse(resp)
}

// parseGeminiResponse extrae texto y/o function calls de la respuesta de Gemini.
func parseGeminiResponse(resp *genai.GenerateContentResponse) (string, []ToolCall, error) {
	calls := resp.FunctionCalls()
	if len(calls) > 0 {
		toolCalls := make([]ToolCall, len(calls))
		for i, fc := range calls {
			toolCalls[i] = ToolCall{Name: fc.Name, Args: fc.Args}
		}
		return "", toolCalls, nil
	}
	return resp.Text(), nil, nil
}

// toGeminiTools convierte []ToolDef al formato nativo de Gemini.
func toGeminiTools(defs []ToolDef) []*genai.Tool {
	decls := make([]*genai.FunctionDeclaration, len(defs))
	for i, d := range defs {
		props := make(map[string]*genai.Schema, len(d.Parameters))
		for _, p := range d.Parameters {
			s := &genai.Schema{
				Type:        toGeminiType(p.Type),
				Description: p.Description,
			}
			if len(p.Enum) > 0 {
				s.Enum = p.Enum
			}
			props[p.Name] = s
		}
		decls[i] = &genai.FunctionDeclaration{
			Name:        d.Name,
			Description: d.Description,
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: props,
				Required:   d.Required,
			},
		}
	}
	return []*genai.Tool{{FunctionDeclarations: decls}}
}

// toGeminiType mapea strings de tipo a genai.Type.
func toGeminiType(t string) genai.Type {
	switch t {
	case "string":
		return genai.TypeString
	case "integer":
		return genai.TypeInteger
	case "number":
		return genai.TypeNumber
	case "boolean":
		return genai.TypeBoolean
	default:
		return genai.TypeString
	}
}

// Transcribe implementa la interfaz LLM. Como estamos usando Groq en producción, 
// dejamos este método preparado por si decides volver a Gemini nativo.
func (g *GeminiLLM) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	return "", fmt.Errorf("transcripción de audio no implementada para Gemini en esta versión")
}
