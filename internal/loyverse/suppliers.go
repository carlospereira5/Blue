package loyverse

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// GetSuppliers obtiene una página de proveedores.
func (c *Client) GetSuppliers(ctx context.Context, limit int, cursor string) (*SuppliersResponse, error) {
	params := url.Values{
		"limit": {fmt.Sprintf("%d", clampLimit(limit))},
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	req, err := c.buildRequest(ctx, http.MethodGet, "/suppliers", params)
	if err != nil {
		return nil, err
	}
	var response SuppliersResponse
	return &response, c.do(req, &response)
}

// GetSupplierByID obtiene un proveedor específico por su ID.
func (c *Client) GetSupplierByID(ctx context.Context, id string) (*Supplier, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/suppliers/"+id, nil)
	if err != nil {
		return nil, err
	}
	var response Supplier
	return &response, c.do(req, &response)
}

// GetAllSuppliers obtiene TODOS los proveedores, manejando la paginación automáticamente.
func (c *Client) GetAllSuppliers(ctx context.Context) ([]Supplier, error) {
	var all []Supplier
	cursor := ""
	for {
		resp, err := c.GetSuppliers(ctx, 250, cursor)
		if err != nil {
			return nil, fmt.Errorf("GetAllSuppliers (cursor=%q): %w", cursor, err)
		}
		all = append(all, resp.Suppliers...)
		if resp.Cursor == "" {
			break
		}
		cursor = resp.Cursor
	}
	return all, nil
}
