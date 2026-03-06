// Package tools contiene todo lo que Aria puede hacer: definición de herramientas,
// ejecución de handlers, acceso a datos y lógica de proveedores.
package tools

import (
	"context"
	"time"

	"aria/internal/db"
	"aria/internal/loyverse"
)

// DataReader es la interfaz unificada de acceso a datos para todos los handlers.
// Abstrae si los datos vienen de la DB local o de la API de Loyverse directamente.
// Naming normalizado: los métodos aquí tienen nombres canónicos independientes
// de los nombres específicos de db.Store o loyverse.Reader.
type DataReader interface {
	GetReceipts(ctx context.Context, since, until time.Time) ([]loyverse.Receipt, error)
	GetShifts(ctx context.Context, since, until time.Time) ([]loyverse.Shift, error)
	GetItems(ctx context.Context) ([]loyverse.Item, error)
	GetCategories(ctx context.Context) ([]loyverse.Category, error)
	GetInventory(ctx context.Context) ([]loyverse.InventoryLevel, error)
	GetPaymentTypes(ctx context.Context) ([]loyverse.PaymentType, error)
	GetEmployees(ctx context.Context) ([]loyverse.Employee, error)
}

// NewFallbackReader crea un DataReader que prefiere la DB local y usa Loyverse
// como fallback. Si store es nil, todas las lecturas van directo a Loyverse.
func NewFallbackReader(store db.Store, loy loyverse.Reader) DataReader {
	return &fallbackReader{db: store, loy: loy}
}

type fallbackReader struct {
	db  db.Store
	loy loyverse.Reader
}

func (r *fallbackReader) GetReceipts(ctx context.Context, since, until time.Time) ([]loyverse.Receipt, error) {
	if r.db != nil {
		return r.db.GetReceiptsByDateRange(ctx, since, until)
	}
	return r.loy.GetAllReceipts(ctx, since, until)
}

func (r *fallbackReader) GetShifts(ctx context.Context, since, until time.Time) ([]loyverse.Shift, error) {
	if r.db != nil {
		return r.db.GetShiftsByDateRange(ctx, since, until)
	}
	return r.loy.GetAllShifts(ctx, since, until)
}

func (r *fallbackReader) GetItems(ctx context.Context) ([]loyverse.Item, error) {
	if r.db != nil {
		return r.db.GetAllItems(ctx)
	}
	return r.loy.GetAllItems(ctx)
}

func (r *fallbackReader) GetCategories(ctx context.Context) ([]loyverse.Category, error) {
	if r.db != nil {
		return r.db.GetAllCategories(ctx)
	}
	resp, err := r.loy.GetCategories(ctx)
	if err != nil {
		return nil, err
	}
	return resp.Categories, nil
}

func (r *fallbackReader) GetInventory(ctx context.Context) ([]loyverse.InventoryLevel, error) {
	if r.db != nil {
		return r.db.GetAllInventoryLevels(ctx)
	}
	return r.loy.GetAllInventory(ctx)
}

func (r *fallbackReader) GetPaymentTypes(ctx context.Context) ([]loyverse.PaymentType, error) {
	if r.db != nil {
		return r.db.GetPaymentTypes(ctx)
	}
	resp, err := r.loy.GetPaymentTypes(ctx)
	if err != nil {
		return nil, err
	}
	return resp.PaymentTypes, nil
}

func (r *fallbackReader) GetEmployees(ctx context.Context) ([]loyverse.Employee, error) {
	if r.db != nil {
		return r.db.GetAllEmployees(ctx)
	}
	resp, err := r.loy.GetAllEmployees(ctx)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
