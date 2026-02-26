package loyverse

import (
	"context"
	"net/http"
)

// GetPaymentTypes obtiene todos los tipos de pago (no paginado).
func (c *Client) GetPaymentTypes(ctx context.Context) (*PaymentTypesResponse, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/payment_types", nil)
	if err != nil {
		return nil, err
	}
	var response PaymentTypesResponse
	return &response, c.do(req, &response)
}

// GetPaymentTypeByID obtiene un tipo de pago específico por su ID.
func (c *Client) GetPaymentTypeByID(ctx context.Context, id string) (*PaymentType, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/payment_types/"+id, nil)
	if err != nil {
		return nil, err
	}
	var response PaymentType
	return &response, c.do(req, &response)
}
