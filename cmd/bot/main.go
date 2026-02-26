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

	"google.golang.org/genai"
)

func main() {
	// Debug logs van a stderr, chat limpio a stdout.
	log.SetOutput(os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	geminiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatalf("gemini client: %v", err)
	}

	loyClient := loyverse.NewClient(http.DefaultClient, cfg.LoyverseAPIKey)

	suppliers, err := agent.LoadSuppliers(cfg.SuppliersFile)
	if err != nil {
		log.Printf("aviso: no se pudo cargar suppliers (%v) — UC5 no va a funcionar", err)
		suppliers = make(map[string][]string)
	}

	lumi := agent.New(geminiClient, loyClient, suppliers, agent.WithDebug(cfg.Debug))

	if cfg.Debug {
		log.Println("[DEBUG] modo debug activado — logs en stderr")
		log.Printf("[DEBUG] LOYVERSE_TOKEN: %s...%s (%d chars)",
			cfg.LoyverseAPIKey[:4], cfg.LoyverseAPIKey[len(cfg.LoyverseAPIKey)-4:], len(cfg.LoyverseAPIKey))
		log.Printf("[DEBUG] GEMINI_API_KEY: %s...%s (%d chars)",
			cfg.GeminiAPIKey[:4], cfg.GeminiAPIKey[len(cfg.GeminiAPIKey)-4:], len(cfg.GeminiAPIKey))
		log.Printf("[DEBUG] SuppliersFile: %s (%d proveedores cargados)", cfg.SuppliersFile, len(suppliers))
	}

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

		// Formatear respuesta con indentación limpia.
		lines := strings.Split(response, "\n")
		for _, line := range lines {
			fmt.Printf("  lumi → %s\n", line)
		}
		fmt.Println()
	}
}
