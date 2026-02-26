package loyverse

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// GetInventory obtiene una página del inventario actual.
func (c *Client) GetInventory(ctx context.Context, limit int, cursor string) (*InventoryResponse, error) {
	params := url.Values{
		"limit": {fmt.Sprintf("%d", clampLimit(limit))},
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	req, err := c.buildRequest(ctx, http.MethodGet, "/inventory", params)
	if err != nil {
		return nil, err
	}
	var response InventoryResponse
	return &response, c.do(req, &response)
}

// GetAllInventory obtiene TODOS los niveles de inventario, manejando paginación.
func (c *Client) GetAllInventory(ctx context.Context) ([]InventoryLevel, error) {
	var all []InventoryLevel
	cursor := ""
	for {
		resp, err := c.GetInventory(ctx, 250, cursor)
		if err != nil {
			return nil, fmt.Errorf("GetAllInventory (cursor=%q): %w", cursor, err)
		}
		all = append(all, resp.Inventories...)
		if resp.Cursor == "" {
			break
		}
		cursor = resp.Cursor
	}
	return all, nil
}
