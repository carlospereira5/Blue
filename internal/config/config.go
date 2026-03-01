// Package config maneja la configuración de la aplicación.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config es la configuración principal de la aplicación.
type Config struct {
	Port             string
	Env              string
	Provider         string // "gemini" o "openai"
	LoyverseAPIKey   string
	GeminiAPIKey     string
	OpenAIAPIKey     string
	OpenAIBaseURL    string
	SuppliersFile    string
	Debug            bool
	WhatsAppDBPath   string
	AllowedNumbers   []string
	WhatsAppGroupJID string
	DBDriver         string // "sqlite" (default) or "postgres"
	DBDSN            string // "blue.db" for SQLite, "postgres://..." for PostgreSQL
	SyncInterval     int    // seconds, default 120
}

// Load carga la configuración desde variables de entorno.
func Load() (*Config, error) {
	loadEnvFiles()

	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		Env:              getEnv("ENV", "development"),
		Provider:         getEnv("PROVIDER", "gemini"),
		LoyverseAPIKey:   getEnv("LOYVERSE_TOKEN", ""),
		GeminiAPIKey:     getEnv("GEMINI_API_KEY", ""),
		OpenAIAPIKey:     getEnv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:    getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		SuppliersFile:    getEnv("SUPPLIERS_FILE", "suppliers.json"),
		Debug:            getEnv("DEBUG", "") == "true",
		WhatsAppDBPath:   getEnv("WHATSAPP_DB_PATH", "whatsapp.db"),
		AllowedNumbers:   parseCSV(getEnv("ALLOWED_NUMBERS", "")),
		WhatsAppGroupJID: getEnv("WHATSAPP_GROUP_JID", ""),
		DBDriver:         getEnv("DB_DRIVER", "sqlite"),
		DBDSN:            getEnv("DB_DSN", "blue.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"),
		SyncInterval:     getEnvInt("SYNC_INTERVAL", 120),
	}

	if cfg.LoyverseAPIKey == "" {
		return nil, fmt.Errorf("LOYVERSE_TOKEN es requerido")
	}

	// Validación estricta por proveedor
	if cfg.Provider == "gemini" && cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY es requerido para provider gemini")
	}
	if cfg.Provider == "openai" && cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY es requerido para provider openai")
	}

	return cfg, nil
}

func loadEnvFiles() {
	var paths []string

	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths,
			filepath.Join(cwd, ".env"),
			filepath.Join(cwd, "..", ".env"),
			filepath.Join(cwd, "..", "..", ".env"),
		)
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		paths = append(paths,
			filepath.Join(exeDir, ".env"),
			filepath.Join(exeDir, "..", ".env"),
		)
	}

	paths = append(paths, ".env", "../.env", "../../.env")

	if _, currentFile, _, ok := runtime.Caller(0); ok {
		rootDir := filepath.Join(filepath.Dir(currentFile), "..", "..", "..")
		paths = append(paths, filepath.Join(rootDir, ".env"))
	}

	_ = godotenv.Load(paths...)
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	s := os.Getenv(key)
	if s == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return v
}

func parseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			result = append(result, v)
		}
	}
	return result
}
