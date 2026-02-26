package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"blue/internal/agent"
	"blue/internal/config"
	"blue/internal/loyverse"
	"blue/internal/whatsapp"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

func main() {
	log.SetOutput(os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var llm agent.LLM

	if cfg.Provider == "openai" {
		apiConfig := openai.DefaultConfig(cfg.OpenAIAPIKey)
		apiConfig.BaseURL = cfg.OpenAIBaseURL
		openaiClient := openai.NewClientWithConfig(apiConfig)
		// Usamos la versión actual soportada por Groq
		llm = agent.NewOpenAILLM(openaiClient, "llama-3.3-70b-versatile")
	} else {
		geminiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  cfg.GeminiAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			log.Fatalf("gemini client: %v", err)
		}
		llm = agent.NewGeminiLLM(geminiClient, "gemini-2.5-flash")
	}

	loyClient := loyverse.NewClient(http.DefaultClient, cfg.LoyverseAPIKey)

	suppliers, err := agent.LoadSuppliers(cfg.SuppliersFile)
	if err != nil {
		log.Printf("aviso: no se pudo cargar suppliers (%v) — UC5 no va a funcionar", err)
		suppliers = make(map[string][]string)
	}

	lumi := agent.New(llm, loyClient, suppliers, agent.WithDebug(cfg.Debug))

	if cfg.Debug {
		log.Println("[DEBUG] modo debug activado — logs en stderr")
		log.Printf("[DEBUG] LOYVERSE_TOKEN: %s...%s (%d chars)",
			cfg.LoyverseAPIKey[:4], cfg.LoyverseAPIKey[len(cfg.LoyverseAPIKey)-4:], len(cfg.LoyverseAPIKey))
		
		if cfg.Provider == "openai" {
			log.Printf("[DEBUG] OPENAI_API_KEY (Groq): %s...%s (%d chars)",
				cfg.OpenAIAPIKey[:4], cfg.OpenAIAPIKey[len(cfg.OpenAIAPIKey)-4:], len(cfg.OpenAIAPIKey))
		} else {
			log.Printf("[DEBUG] GEMINI_API_KEY: %s...%s (%d chars)",
				cfg.GeminiAPIKey[:4], cfg.GeminiAPIKey[len(cfg.GeminiAPIKey)-4:], len(cfg.GeminiAPIKey))
		}
		log.Printf("[DEBUG] SuppliersFile: %s (%d proveedores cargados)", cfg.SuppliersFile, len(suppliers))
	}

	if len(cfg.AllowedNumbers) > 0 {
		runWhatsApp(ctx, lumi, cfg)
	} else {
		runCLI(ctx, lumi)
	}
}

func runWhatsApp(ctx context.Context, lumi *agent.Agent, cfg *config.Config) {
	log.Printf("[whatsapp] iniciando con %d número(s) autorizado(s)", len(cfg.AllowedNumbers))

	bot, err := whatsapp.New(ctx, lumi, cfg.WhatsAppDBPath, cfg.AllowedNumbers, cfg.WhatsAppGroupJID)
	if err != nil {
		log.Fatalf("whatsapp: %v", err)
	}

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════╗")
	fmt.Println("  ║   Lumi — WhatsApp Bot            ║")
	fmt.Println("  ╚══════════════════════════════════╝")
	fmt.Println()

	if err := bot.Start(ctx); err != nil {
		log.Fatalf("whatsapp: %v", err)
	}
}

func runCLI(ctx context.Context, lumi *agent.Agent) {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════╗")
	fmt.Println("  ║   Lumi — Asistente del kiosco    ║")
	fmt.Println("  ╚══════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  Escribí tu pregunta. 'salir' para terminar.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("  vos → ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "salir" || input == "exit" {
			fmt.Println()
			fmt.Println("  ¡Chau!")
			fmt.Println()
			break
		}

		fmt.Println()
		response, chatErr := lumi.Chat(ctx, input)
		if chatErr != nil {
			fmt.Printf("  ⚠ Error: %v\n\n", chatErr)
			continue
		}

		lines := strings.Split(response, "\n")
		for _, line := range lines {
			fmt.Printf("  lumi → %s\n", line)
		}
		fmt.Println()
	}
}
