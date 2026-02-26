package loyverse

import (
	"context"
	"net/http"
)

// GetCategories obtiene todas las categorías (no paginado).
func (c *Client) GetCategories(ctx context.Context) (*CategoriesResponse, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/categories", nil)
	if err != nil {
		return nil, err
	}
	var response CategoriesResponse
	return &response, c.do(req, &response)
}
