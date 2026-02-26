package loyverse

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// GetShifts obtiene una página de turnos en el rango de fechas dado.
// Usa created_at_min/created_at_max como indica la API de Loyverse.
func (c *Client) GetShifts(ctx context.Context, since, until time.Time, limit int, cursor string) (*ShiftsResponse, error) {
	params := url.Values{
		"created_at_min": {loyverseDate(since)},
		"created_at_max": {loyverseDate(until)},
		"limit":          {fmt.Sprintf("%d", clampLimit(limit))},
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	req, err := c.buildRequest(ctx, http.MethodGet, "/shifts", params)
	if err != nil {
		return nil, err
	}
	var response ShiftsResponse
	return &response, c.do(req, &response)
}

// GetShiftByID obtiene un shift específico por su ID.
func (c *Client) GetShiftByID(ctx context.Context, id string) (*Shift, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/shifts/"+id, nil)
	if err != nil {
		return nil, err
	}
	var response Shift
	return &response, c.do(req, &response)
}

// GetAllShifts obtiene TODOS los shifts en el rango, manejando la paginación automáticamente.
func (c *Client) GetAllShifts(ctx context.Context, since, until time.Time) ([]Shift, error) {
	var all []Shift
	cursor := ""
	for {
		resp, err := c.GetShifts(ctx, since, until, 250, cursor)
		if err != nil {
			return nil, fmt.Errorf("GetAllShifts (cursor=%q): %w", cursor, err)
		}
		all = append(all, resp.Shifts...)
		if resp.Cursor == "" {
			break
		}
		cursor = resp.Cursor
	}
	return all, nil
}
