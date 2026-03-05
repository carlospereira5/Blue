package llm

import (
	"bytes"
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

// Transcribe envía el audio OGG de WhatsApp al endpoint Whisper de Groq.
func (o *OpenAILLM) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	req := openai.AudioRequest{
		Model:    "whisper-large-v3-turbo",
		Reader:   bytes.NewReader(audioData),
		FilePath: "voice_note.ogg",
		Format:   openai.AudioResponseFormatText,
	}
	resp, err := o.client.CreateTranscription(ctx, req)
	if err != nil {
		return "", fmt.Errorf("openai transcription: %w", err)
	}
	return resp.Text, nil
}

func (o *OpenAILLM) NewSession(_ context.Context, systemPrompt string, tools []ToolDef) (Session, error) {
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
		schema := map[string]any{"type": "object", "properties": props}
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

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}
	return &openAISession{
		client:          o.client,
		model:           o.model,
		messages:        messages,
		tools:           oTools,
		lastToolCallIDs: make(map[string]string),
	}, nil
}

type openAISession struct {
	client          *openai.Client
	model           string
	messages        []openai.ChatCompletionMessage
	tools           []openai.Tool
	lastToolCallIDs map[string]string
}

func (s *openAISession) Send(ctx context.Context, message string) (string, []ToolCall, error) {
	s.messages = append(s.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	})
	return s.do(ctx)
}

func (s *openAISession) SendToolResults(ctx context.Context, results []ToolResult) (string, []ToolCall, error) {
	for _, r := range results {
		contentBytes, err := json.Marshal(r.Result)
		if err != nil {
			return "", nil, fmt.Errorf("marshaling tool result: %w", err)
		}
		s.messages = append(s.messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    string(contentBytes),
			Name:       r.Name,
			ToolCallID: s.lastToolCallIDs[r.Name],
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
	s.messages = append(s.messages, msg)

	if len(msg.ToolCalls) > 0 {
		toolCalls := make([]ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			s.lastToolCallIDs[tc.Function.Name] = tc.ID
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return "", nil, fmt.Errorf("parsing tool args: %w", err)
			}
			toolCalls[i] = ToolCall{Name: tc.Function.Name, Args: args}
		}
		return "", toolCalls, nil
	}
	return msg.Content, nil, nil
}
