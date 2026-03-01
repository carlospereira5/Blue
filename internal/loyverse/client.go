// Package loyverse contiene el cliente HTTP para la API de Loyverse v1.0.
package loyverse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://api.loyverse.com/v1.0"

// HTTPClient define la interfaz para el cliente HTTP. Permite inyectar un mock en tests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Reader define las operaciones de lectura disponibles contra la API de Loyverse.
type Reader interface {
	GetReceipts(ctx context.Context, since, until time.Time, limit int, cursor string) (*ReceiptsResponse, error)
	GetReceiptByID(ctx context.Context, id string) (*Receipt, error)
	GetAllReceipts(ctx context.Context, since, until time.Time) ([]Receipt, error)
	GetItems(ctx context.Context, limit int, cursor string) (*ItemsResponse, error)
	GetItemByID(ctx context.Context, id string) (*Item, error)
	GetAllItems(ctx context.Context) ([]Item, error)
	GetCategories(ctx context.Context) (*CategoriesResponse, error)
	GetInventory(ctx context.Context, limit int, cursor string) (*InventoryResponse, error)
	GetAllInventory(ctx context.Context) ([]InventoryLevel, error)
	GetShifts(ctx context.Context, since, until time.Time, limit int, cursor string) (*ShiftsResponse, error)
	GetShiftByID(ctx context.Context, id string) (*Shift, error)
	GetAllShifts(ctx context.Context, since, until time.Time) ([]Shift, error)
	GetEmployees(ctx context.Context, limit int, cursor string) (*EmployeesResponse, error)
	GetEmployeeByID(ctx context.Context, id string) (*Employee, error)
	GetStores(ctx context.Context) (*StoresResponse, error)
	GetStoreByID(ctx context.Context, id string) (*Store, error)
	GetPaymentTypes(ctx context.Context) (*PaymentTypesResponse, error)
	GetPaymentTypeByID(ctx context.Context, id string) (*PaymentType, error)
	GetSuppliers(ctx context.Context, limit int, cursor string) (*SuppliersResponse, error)
	GetSupplierByID(ctx context.Context, id string) (*Supplier, error)
	ItemNameToID(ctx context.Context) (map[string]string, error)
}

// Client es el cliente HTTP para la API de Loyverse.
type Client struct {
	httpClient HTTPClient
	baseURL    string
	token      string
}

// Option es una función de configuración para el Client.
type Option func(*Client)

// WithBaseURL sobreescribe la base URL del cliente. Útil para tests.
func WithBaseURL(u string) Option {
	return func(c *Client) {
		c.baseURL = u
	}
}

// NewClient crea un nuevo Client listo para usar.
// Si httpClient es nil, usa http.DefaultClient.
func NewClient(httpClient HTTPClient, token string, opts ...Option) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	c := &Client{
		httpClient: httpClient,
		baseURL:    defaultBaseURL,
		token:      token,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// buildRequest construye un request HTTP con auth Bearer y query params.
func (c *Client) buildRequest(ctx context.Context, method, endpoint string, params url.Values) (*http.Request, error) {
	u := c.baseURL + endpoint
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// do ejecuta el request, verifica el status HTTP y parsea el JSON en dest.
func (c *Client) do(req *http.Request, dest any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	// DEBUG: log response body for receipts endpoint
	if req.URL.Path == "/receipts" && len(body) > 0 {
		// Log first 500 chars to avoid flooding
		maxLen := 500
		if len(body) < maxLen {
			maxLen = len(body)
		}
		log.Printf("[DEBUG loyverse] /receipts response (first %d bytes): %s", maxLen, string(body[:maxLen]))
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("loyverse API error %d: %s", resp.StatusCode, string(body))
	}

	return json.Unmarshal(body, dest)
}

// Formato de fecha que usa Loyverse en su API.
const loyverseTimeFormat = "2006-01-02T15:04:05.000Z"

func loyverseDate(t time.Time) string { return t.UTC().Format(loyverseTimeFormat) }

// clampLimit asegura que el limit esté en el rango válido de Loyverse (1-250).
func clampLimit(n int) int {
	if n <= 0 || n > 250 {
		return 250
	}
	return n
}
