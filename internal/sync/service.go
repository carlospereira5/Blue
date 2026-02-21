// Package sync implementa la sincronización de datos entre Loyverse y la DB de Blue.
package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	"blue/internal/loyverse"
	"blue/internal/repository"
)

// LoyverseReader define las operaciones de lectura que el sync service necesita.
// Interfaz definida en el sitio del consumidor (este package).
type LoyverseReader interface {
	GetAllItems(ctx context.Context) ([]loyverse.Item, error)
	GetCategories(ctx context.Context) (*loyverse.CategoriesResponse, error)
	GetAllReceipts(ctx context.Context, since, until time.Time) ([]loyverse.Receipt, error)
}

// Service orquesta la sincronización inicial y el catch-up de receipts.
type Service struct {
	lv         LoyverseReader
	items      repository.ItemWriter
	receipts   repository.ReceiptWriter
	syncCursor repository.SyncCursorStore
}

// New crea un nuevo Service de sincronización.
func New(
	lv LoyverseReader,
	items repository.ItemWriter,
	receipts repository.ReceiptWriter,
	syncCursor repository.SyncCursorStore,
) *Service {
	return &Service{
		lv:         lv,
		items:      items,
		receipts:   receipts,
		syncCursor: syncCursor,
	}
}

// InitialSync sincroniza el catálogo completo: categorías primero, luego items.
// Llama a esto cuando el cursor de items está en zero (primera ejecución).
func (s *Service) InitialSync(ctx context.Context) error {
	log.Println("sync: iniciando sincronización inicial del catálogo...")

	cats, err := s.lv.GetCategories(ctx)
	if err != nil {
		return fmt.Errorf("InitialSync GetCategories: %w", err)
	}

	for _, cat := range cats.Categories {
		if err := s.items.UpsertCategory(ctx, cat); err != nil {
			return fmt.Errorf("InitialSync UpsertCategory %q: %w", cat.ID, err)
		}
	}
	log.Printf("sync: %d categorías sincronizadas", len(cats.Categories))

	allItems, err := s.lv.GetAllItems(ctx)
	if err != nil {
		return fmt.Errorf("InitialSync GetAllItems: %w", err)
	}

	for _, item := range allItems {
		if err := s.items.UpsertItem(ctx, item); err != nil {
			return fmt.Errorf("InitialSync UpsertItem %q: %w", item.ID, err)
		}
	}
	log.Printf("sync: %d items sincronizados", len(allItems))

	if err := s.syncCursor.SetSyncCursor(ctx, repository.EntityItems, time.Now()); err != nil {
		return fmt.Errorf("InitialSync SetSyncCursor items: %w", err)
	}

	log.Println("sync: sincronización inicial completa")
	return nil
}

// CatchUpReceipts sincroniza receipts desde el último cursor hasta ahora.
// Si no hay cursor previo, retrocede 90 días como punto de partida seguro.
//
// Nota: usa created_at_min en la API de Loyverse. En v2, cuando el cliente
// soporte updated_at_min, cambiar para capturar edits y refunds en syncs incrementales.
func (s *Service) CatchUpReceipts(ctx context.Context) error {
	since, err := s.syncCursor.GetSyncCursor(ctx, repository.EntityReceipts)
	if err != nil {
		return fmt.Errorf("CatchUpReceipts GetSyncCursor: %w", err)
	}

	if since.IsZero() {
		since = time.Now().AddDate(0, 0, -90)
		log.Printf("sync: sin cursor de receipts — arrancando desde %s", since.Format(time.DateOnly))
	}

	until := time.Now()
	log.Printf("sync: catch-up receipts desde %s hasta %s", since.Format(time.RFC3339), until.Format(time.RFC3339))

	receipts, err := s.lv.GetAllReceipts(ctx, since, until)
	if err != nil {
		return fmt.Errorf("CatchUpReceipts GetAllReceipts: %w", err)
	}

	for _, r := range receipts {
		if err := s.receipts.UpsertReceipt(ctx, r); err != nil {
			// Log pero no abortar — un receipt con FK error no debe bloquear el resto
			log.Printf("WARN CatchUpReceipts UpsertReceipt %q: %v", r.ID, err)
		}
	}

	if err := s.syncCursor.SetSyncCursor(ctx, repository.EntityReceipts, until); err != nil {
		return fmt.Errorf("CatchUpReceipts SetSyncCursor: %w", err)
	}

	log.Printf("sync: catch-up completo — %d receipts procesados", len(receipts))
	return nil
}
