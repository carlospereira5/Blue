package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"blue/internal/loyverse"

	"google.golang.org/genai"
)

const modelName = "gemini-2.5-flash"

// Agent es el cerebro del chatbot Lumi. Conecta Gemini con Loyverse
// usando function calling para responder consultas en lenguaje natural.
type Agent struct {
	client    *genai.Client
	loyverse  loyverse.Reader
	suppliers map[string][]string
	debug     bool
}

// Option configura el Agent.
type Option func(*Agent)

// WithDebug activa el modo debug que logea todo el flujo interno.
func WithDebug(enabled bool) Option {
	return func(a *Agent) { a.debug = enabled }
}

// New crea un nuevo Agent listo para chatear.
func New(client *genai.Client, loy loyverse.Reader, suppliers map[string][]string, opts ...Option) *Agent {
	a := &Agent{
		client:    client,
		loyverse:  loy,
		suppliers: suppliers,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *Agent) debugLog(format string, args ...any) {
	if a.debug {
		log.Printf("[DEBUG agent] "+format, args...)
	}
}

// Chat envía un mensaje al modelo, ejecuta el loop de function calling,
// y retorna la respuesta de texto final.
// Cada llamada a Chat() es independiente (no se persiste historia).
func (a *Agent) Chat(ctx context.Context, message string) (string, error) {
	a.debugLog(">>> mensaje usuario: %q", message)

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(systemPrompt)},
			Role:  "user",
		},
		Tools:       lumiTools(),
		Temperature: genai.Ptr[float32](0.3),
	}

	chat, err := a.client.Chats.Create(ctx, modelName, config, nil)
	if err != nil {
		return "", fmt.Errorf("creating chat session: %w", err)
	}
	a.debugLog("chat session creada con modelo %s", modelName)

	resp, err := chat.Send(ctx, genai.NewPartFromText(message))
	if err != nil {
		return "", fmt.Errorf("sending message: %w", err)
	}
	a.debugLog("respuesta inicial de Gemini recibida")
	a.debugResponse(resp)

	// Function calling loop — max 5 iteraciones como safety.
	for i := 0; i < 5; i++ {
		calls := resp.FunctionCalls()
		if len(calls) == 0 {
			a.debugLog("no hay function calls — saliendo del loop (iteración %d)", i)
			break
		}

		a.debugLog("--- iteración %d: %d function call(s) ---", i, len(calls))

		var responseParts []*genai.Part
		for _, fc := range calls {
			a.debugLog("TOOL CALL: %s(%s)", fc.Name, mustJSONString(fc.Args))

			result, execErr := a.ExecuteTool(ctx, fc.Name, fc.Args)
			if execErr != nil {
				a.debugLog("TOOL ERROR: %s → %v", fc.Name, execErr)
				result = map[string]any{"error": execErr.Error()}
			} else {
				a.debugLog("TOOL RESULT: %s → %s", fc.Name, truncate(mustJSONString(result), 500))
			}
			responseParts = append(responseParts, genai.NewPartFromFunctionResponse(fc.Name, result))
		}

		resp, err = chat.Send(ctx, responseParts...)
		if err != nil {
			return "", fmt.Errorf("sending function response: %w", err)
		}
		a.debugLog("respuesta de Gemini tras function response")
		a.debugResponse(resp)
	}

	text := resp.Text()
	a.debugLog("<<< respuesta final: %s", truncate(text, 200))
	if text == "" {
		return "No pude generar una respuesta. Intentá reformular tu pregunta.", nil
	}
	return text, nil
}

func (a *Agent) debugResponse(resp *genai.GenerateContentResponse) {
	if !a.debug || resp == nil {
		return
	}
	for i, c := range resp.Candidates {
		if c.Content == nil {
			a.debugLog("  candidate[%d]: content=nil", i)
			continue
		}
		for j, p := range c.Content.Parts {
			if p.Text != "" {
				a.debugLog("  candidate[%d].part[%d]: TEXT=%s", i, j, truncate(p.Text, 200))
			}
			if p.FunctionCall != nil {
				a.debugLog("  candidate[%d].part[%d]: FUNCTION_CALL=%s(%s)", i, j, p.FunctionCall.Name, mustJSONString(p.FunctionCall.Args))
			}
			if p.FunctionResponse != nil {
				a.debugLog("  candidate[%d].part[%d]: FUNCTION_RESPONSE=%s", i, j, p.FunctionResponse.Name)
			}
		}
	}
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
