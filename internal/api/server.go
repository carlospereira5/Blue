package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Server encapsula el router HTTP de Blue.
type Server struct {
	router *gin.Engine
}

// NewServer crea y configura el servidor HTTP.
// Usa gin.New() en lugar de gin.Default() para control explícito de middlewares.
func NewServer(db *sql.DB, webhookSecret string, processor ReceiptProcessor) *Server {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	router.GET("/health", healthHandler(db))
	router.POST("/webhooks/loyverse/:secret", webhookHandler(webhookSecret, processor))

	return &Server{router: router}
}

// Handler retorna el http.Handler para usar con http.Server.
// Permite control externo del graceful shutdown.
func (s *Server) Handler() http.Handler {
	return s.router
}
