package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAILLM implementa LLM usando el SDK compatible con OpenAI/Groq.
type OpenAILLM struct {
	client *openai.Client
	model  string
}

func NewOpenAILLM(client *openai.Client, model string) *OpenAILLM {
	return &OpenAILLM{client: client, model: model}
}

func (o *OpenAILLM) NewSession(ctx context.Context, systemPrompt string, tools []ToolDef) (Session, error) {
	oTools := make([]openai.Tool, len(tools))
	for i, t := range tools {
		props := make(map[string]any)
		for _, p := range t.Parameters {
			prop := map[string]any{"type": p.Type, "description": p.Description}
			if len(p.Enum) > 0 {
				prop["enum"] = p.Enum
			}
			props[p.Name] = prop
		}
		
		schema := map[string]any{
			"type":       "object",
			"properties": props,
		}

		// FIX: Si Required es nil o está vacío, lo omitimos del mapa. 
		// Esto evita que el marshaler de Go genere un "null" literal que rompe 
		// la validación estricta de JSON Schema (Draft 2020-12) en la API de Groq.
		if len(t.Required) > 0 {
			schema["required"] = t.Required
		}

		schemaBytes, err := json.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("marshaling tool schema: %w", err)
		}

		oTools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(schemaBytes),
			},
		}
	}

	// Preasignación de capacidad para evitar fragmentación de memoria en el heap (slice growth).
	messages := make([]openai.ChatCompletionMessage, 0, 16)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})

	return &openAISession{
		client:          o.client,
		model:           o.model,
		messages:        messages,
		tools:           oTools,
		lastToolCallIDs: nil,
	}, nil
}

type openAISession struct {
	client          *openai.Client
	model           string
	messages        []openai.ChatCompletionMessage
	tools           []openai.Tool
	lastToolCallIDs []string // IDs por índice, paralelo a []ToolCall retornado
}

func (s *openAISession) Send(ctx context.Context, message string) (string, []ToolCall, error) {
	s.messages = append(s.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	})
	return s.do(ctx)
}

func (s *openAISession) SendToolResults(ctx context.Context, results []ToolResult) (string, []ToolCall, error) {
	for i, r := range results {
		contentBytes, err := json.Marshal(r.Result)
		if err != nil {
			return "", nil, fmt.Errorf("marshaling tool result: %w", err)
		}
		toolCallID := ""
		if i < len(s.lastToolCallIDs) {
			toolCallID = s.lastToolCallIDs[i]
		}
		s.messages = append(s.messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    string(contentBytes),
			Name:       r.Name,
			ToolCallID: toolCallID,
		})
	}
	return s.do(ctx)
}

func (s *openAISession) do(ctx context.Context) (string, []ToolCall, error) {
	req := openai.ChatCompletionRequest{
		Model:       s.model,
		Messages:    s.messages,
		Tools:       s.tools,
		Temperature: 0.3,
	}

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", nil, fmt.Errorf("openai chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("openai devolvió 0 choices")
	}

	msg := resp.Choices[0].Message
	// Almacenar respuesta del asistente para mantener contexto en memoria
	s.messages = append(s.messages, msg)

	if len(msg.ToolCalls) > 0 {
		toolCalls := make([]ToolCall, len(msg.ToolCalls))
		s.lastToolCallIDs = make([]string, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			s.lastToolCallIDs[i] = tc.ID

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return "", nil, fmt.Errorf("parsing tool args: %w", err)
			}
			toolCalls[i] = ToolCall{
				Name: tc.Function.Name,
				Args: args,
			}
		}
		return "", toolCalls, nil
	}

	return msg.Content, nil, nil
}
