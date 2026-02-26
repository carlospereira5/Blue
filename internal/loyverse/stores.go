package loyverse

import (
	"context"
	"net/http"
)

// GetStores obtiene todas las tiendas (no paginado).
func (c *Client) GetStores(ctx context.Context) (*StoresResponse, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/stores", nil)
	if err != nil {
		return nil, err
	}
	var response StoresResponse
	return &response, c.do(req, &response)
}

// GetStoreByID obtiene una tienda específica por su ID.
func (c *Client) GetStoreByID(ctx context.Context, id string) (*Store, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/stores/"+id, nil)
	if err != nil {
		return nil, err
	}
	var response Store
	return &response, c.do(req, &response)
}
