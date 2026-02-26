package loyverse

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// GetReceipts obtiene una página de receipts en el rango de fechas dado.
func (c *Client) GetReceipts(ctx context.Context, since, until time.Time, limit int, cursor string) (*ReceiptsResponse, error) {
	params := url.Values{
		"created_at_min": {loyverseDate(since)},
		"created_at_max": {loyverseDate(until)},
		"limit":          {fmt.Sprintf("%d", clampLimit(limit))},
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	req, err := c.buildRequest(ctx, http.MethodGet, "/receipts", params)
	if err != nil {
		return nil, err
	}
	var response ReceiptsResponse
	return &response, c.do(req, &response)
}

// GetReceiptByID obtiene un receipt específico por su ID.
func (c *Client) GetReceiptByID(ctx context.Context, id string) (*Receipt, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/receipts/"+id, nil)
	if err != nil {
		return nil, err
	}
	var response Receipt
	return &response, c.do(req, &response)
}

// GetAllReceipts obtiene TODOS los receipts en el rango, manejando la paginación automáticamente.
func (c *Client) GetAllReceipts(ctx context.Context, since, until time.Time) ([]Receipt, error) {
	var all []Receipt
	cursor := ""
	for {
		resp, err := c.GetReceipts(ctx, since, until, 250, cursor)
		if err != nil {
			return nil, fmt.Errorf("GetAllReceipts (cursor=%q): %w", cursor, err)
		}
		all = append(all, resp.Receipts...)
		if resp.Cursor == "" {
			break
		}
		cursor = resp.Cursor
	}
	return all, nil
}
