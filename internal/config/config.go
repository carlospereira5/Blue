// Package config maneja la configuración de la aplicación.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
)

// Config es la configuración principal de la aplicación.
type Config struct {
	Port             string
	Env              string
	LoyverseAPIKey   string
	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string
	WebhookSecret    string
}

// Load carga la configuración desde variables de entorno.
// Automáticamente busca archivos .env en múltiples ubicaciones.
func Load() (*Config, error) {
	loadEnvFiles()

	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		Env:              getEnv("ENV", "development"),
		LoyverseAPIKey:   getEnv("LOYVERSE_TOKEN", ""),
		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresUser:     getEnv("POSTGRES_USER", "kiosko"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", ""),
		PostgresDB:       getEnv("POSTGRES_DB", "blue"),
		PostgresSSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		WebhookSecret:    getEnv("WEBHOOK_SECRET", ""),
	}

	if cfg.LoyverseAPIKey == "" {
		return nil, fmt.Errorf("LOYVERSE_TOKEN es requerido")
	}

	return cfg, nil
}

// DSN retorna el connection string de PostgreSQL.
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.PostgresHost, c.PostgresPort, c.PostgresUser,
		c.PostgresPassword, c.PostgresDB, c.PostgresSSLMode)
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
