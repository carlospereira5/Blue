package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"blue/internal/loyverse"
)

// ReceiptProcessor define lo que el webhook necesita para procesar un receipt.
// Interfaz definida en el sitio del consumidor (este package).
type ReceiptProcessor interface {
	UpsertReceipt(ctx context.Context, r loyverse.Receipt) error
}

// webhookPayload es el envelope que Loyverse envía en receipts.update.
type webhookPayload struct {
	MerchantID string            `json:"merchant_id"`
	Type       string            `json:"type"`
	CreatedAt  time.Time         `json:"created_at"`
	Receipts   []loyverse.Receipt `json:"receipts"`
}

// webhookHandler procesa eventos de Loyverse en tiempo real.
//
// Reglas críticas:
//  1. ACK 200 INMEDIATAMENTE — antes de cualquier lógica. Loyverse reintenta 200 veces
//     en 48h ante cualquier non-2xx. Si falla, auto-DISABLE del webhook.
//  2. Validar el secret como path component (no header) — token estático de Loyverse
//     no incluye X-Loyverse-Signature. Retornar 404 para no revelar existencia.
//  3. JSON inválido → 200 igual — payload roto no debe disparar retry infinito.
//  4. Procesar async en goroutine con context propio — el ctx del request ya está
//     cancelado cuando el handler retorna.
func webhookHandler(secret string, processor ReceiptProcessor) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Validar secret — 404 si no coincide (no revelar existencia del endpoint)
		if c.Param("secret") != secret {
			c.Status(http.StatusNotFound)
			return
		}

		// 2. Leer body
		var payload webhookPayload
		if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
			// JSON inválido → ACK 200 igual para evitar retry infinito
			log.Printf("WARN webhook: JSON inválido — %v", err)
			c.Status(http.StatusOK)
			return
		}

		// 3. ACK INMEDIATO — Loyverse necesita 200 antes de que procesemos
		c.Status(http.StatusOK)

		// 4. Procesar async con context propio (el del request ya estará cancelado)
		go func(receipts []loyverse.Receipt) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			for _, r := range receipts {
				if err := processor.UpsertReceipt(ctx, r); err != nil {
					log.Printf("ERROR webhook UpsertReceipt %q: %v", r.ID, err)
				}
			}
		}(payload.Receipts)
	}
}
