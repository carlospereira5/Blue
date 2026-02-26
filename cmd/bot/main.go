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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Gemini client
	geminiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatalf("gemini client: %v", err)
	}

	// Loyverse client
	loyClient := loyverse.NewClient(http.DefaultClient, cfg.LoyverseAPIKey)

	// Suppliers
	suppliers, err := agent.LoadSuppliers(cfg.SuppliersFile)
	if err != nil {
		log.Printf("aviso: no se pudo cargar suppliers (%v) — UC5 no va a funcionar", err)
		suppliers = make(map[string][]string)
	}

	lumi := agent.New(geminiClient, loyClient, suppliers, agent.WithDebug(cfg.Debug))

	if cfg.Debug {
		log.Println("[DEBUG] modo debug activado — se loguea todo el flujo interno")
		log.Printf("[DEBUG] LOYVERSE_TOKEN: %s...%s (%d chars)",
			cfg.LoyverseAPIKey[:4], cfg.LoyverseAPIKey[len(cfg.LoyverseAPIKey)-4:], len(cfg.LoyverseAPIKey))
		log.Printf("[DEBUG] GEMINI_API_KEY: %s...%s (%d chars)",
			cfg.GeminiAPIKey[:4], cfg.GeminiAPIKey[len(cfg.GeminiAPIKey)-4:], len(cfg.GeminiAPIKey))
		log.Printf("[DEBUG] SuppliersFile: %s (%d proveedores cargados)", cfg.SuppliersFile, len(suppliers))
	}

	fmt.Println("🤖 Lumi — Asistente del kiosco")
	fmt.Println("Escribí tu pregunta y presioná Enter. 'salir' para terminar.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("vos> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "salir" || input == "exit" {
			fmt.Println("¡Chau!")
			break
		}

		response, chatErr := lumi.Chat(ctx, input)
		if chatErr != nil {
			fmt.Printf("error: %v\n\n", chatErr)
			continue
		}
		fmt.Printf("\nlumi> %s\n\n", response)
	}
}
