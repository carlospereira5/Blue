// Package config maneja la configuración de la aplicación.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
)

// Config es la configuración principal de la aplicación.
type Config struct {
	Port           string
	Env            string
	LoyverseAPIKey string
	GeminiAPIKey   string
	SuppliersFile  string
	Debug          bool
	WhatsAppDBPath string
	AllowedNumbers []string
}

// Load carga la configuración desde variables de entorno.
// Automáticamente busca archivos .env en múltiples ubicaciones.
func Load() (*Config, error) {
	loadEnvFiles()

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		Env:            getEnv("ENV", "development"),
		LoyverseAPIKey: getEnv("LOYVERSE_TOKEN", ""),
		GeminiAPIKey:   getEnv("GEMINI_API_KEY", ""),
		SuppliersFile:  getEnv("SUPPLIERS_FILE", "suppliers.json"),
		Debug:          getEnv("DEBUG", "") == "true",
		WhatsAppDBPath: getEnv("WHATSAPP_DB_PATH", "whatsapp.db"),
		AllowedNumbers: parseCSV(getEnv("ALLOWED_NUMBERS", "")),
	}

	if cfg.LoyverseAPIKey == "" {
		return nil, fmt.Errorf("LOYVERSE_TOKEN es requerido")
	}
	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY es requerido")
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
