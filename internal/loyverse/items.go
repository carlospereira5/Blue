package loyverse

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetItems obtiene una página del catálogo de productos.
func (c *Client) GetItems(ctx context.Context, limit int, cursor string) (*ItemsResponse, error) {
	params := url.Values{
		"limit": {fmt.Sprintf("%d", clampLimit(limit))},
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	req, err := c.buildRequest(ctx, http.MethodGet, "/items", params)
	if err != nil {
		return nil, err
	}
	var response ItemsResponse
	return &response, c.do(req, &response)
}

// GetItemByID obtiene un item específico por su ID.
func (c *Client) GetItemByID(ctx context.Context, id string) (*Item, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/items/"+id, nil)
	if err != nil {
		return nil, err
	}
	var response Item
	return &response, c.do(req, &response)
}

// GetAllItems obtiene TODOS los items del catálogo, manejando la paginación automáticamente.
func (c *Client) GetAllItems(ctx context.Context) ([]Item, error) {
	var all []Item
	cursor := ""
	for {
		resp, err := c.GetItems(ctx, 250, cursor)
		if err != nil {
			return nil, fmt.Errorf("GetAllItems (cursor=%q): %w", cursor, err)
		}
		all = append(all, resp.Items...)
		if resp.Cursor == "" {
			break
		}
		cursor = resp.Cursor
	}
	return all, nil
}

// ItemNameToID retorna un mapa nombre→ID para todos los items del catálogo.
// Incluye tanto el nombre exacto como la versión en minúsculas para búsquedas flexibles.
func (c *Client) ItemNameToID(ctx context.Context) (map[string]string, error) {
	items, err := c.GetAllItems(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(items)*2)
	for _, item := range items {
		result[item.ItemName] = item.ID
		result[strings.ToLower(item.ItemName)] = item.ID
	}
	return result, nil
}
