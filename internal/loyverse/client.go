// Package loyverse contiene el cliente HTTP para la API de Loyverse v1.0.
package loyverse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
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
	GetItems(ctx context.Context, limit int, cursor string) (*ItemsResponse, error)
	GetItemByID(ctx context.Context, id string) (*Item, error)
	GetCategories(ctx context.Context) (*CategoriesResponse, error)
	GetInventory(ctx context.Context, limit int, cursor string) (*InventoryResponse, error)
	GetShifts(ctx context.Context, since, until time.Time, limit int, cursor string) (*ShiftsResponse, error)
}

// SortOption y SortOrder controlan el ordenamiento de listas.
type (
	SortOption string
	SortOrder  string
)

const (
	SortByName       SortOption = "name"
	SortByCategory   SortOption = "category"
	SortByPrice      SortOption = "price"
	SortByDate       SortOption = "date"
	SortByTotal      SortOption = "total"
	SortByReceiptNum SortOption = "receipt_number"
	SortAsc          SortOrder  = "asc"
	SortDesc         SortOrder  = "desc"
)

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
	var response singleReceiptResponse
	return &response.Receipt, c.do(req, &response)
}

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
	var response singleItemResponse
	return &response.Item, c.do(req, &response)
}

// GetCategories obtiene todas las categorías (no paginado).
func (c *Client) GetCategories(ctx context.Context) (*CategoriesResponse, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/categories", nil)
	if err != nil {
		return nil, err
	}
	var response CategoriesResponse
	return &response, c.do(req, &response)
}

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

// GetShifts obtiene una página de turnos en el rango de fechas dado.
func (c *Client) GetShifts(ctx context.Context, since, until time.Time, limit int, cursor string) (*ShiftsResponse, error) {
	params := url.Values{
		"opened_at_min": {loyverseDate(since)},
		"opened_at_max": {loyverseDate(until)},
		"limit":         {fmt.Sprintf("%d", clampLimit(limit))},
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

// reverse invierte un slice in-place.
func reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// SortItems ordena items por el criterio y orden especificados.
func SortItems(items []Item, by SortOption, order SortOrder) {
	switch by {
	case SortByName:
		sort.Slice(items, func(i, j int) bool {
			return strings.ToLower(items[i].ItemName) < strings.ToLower(items[j].ItemName)
		})
	case SortByPrice:
		sort.Slice(items, func(i, j int) bool {
			return items[i].EffectivePrice() < items[j].EffectivePrice()
		})
	case SortByCategory:
		sort.Slice(items, func(i, j int) bool {
			return items[i].CategoryID < items[j].CategoryID
		})
	}
	if order == SortDesc {
		reverse(items)
	}
}

// SortReceipts ordena receipts por el criterio y orden especificados.
func SortReceipts(receipts []Receipt, by SortOption, order SortOrder) {
	switch by {
	case SortByDate:
		sort.Slice(receipts, func(i, j int) bool {
			return receipts[i].CreatedAt.Before(receipts[j].CreatedAt)
		})
	case SortByTotal:
		sort.Slice(receipts, func(i, j int) bool {
			return receipts[i].ReceiptTotal < receipts[j].ReceiptTotal
		})
	case SortByReceiptNum:
		sort.Slice(receipts, func(i, j int) bool {
			return receipts[i].ReceiptNumber < receipts[j].ReceiptNumber
		})
	}
	if order == SortDesc {
		reverse(receipts)
	}
}

// SortCategories ordena categorías alfabéticamente.
func SortCategories(categories []Category, order SortOrder) {
	sort.Slice(categories, func(i, j int) bool {
		return strings.ToLower(categories[i].Name) < strings.ToLower(categories[j].Name)
	})
	if order == SortDesc {
		reverse(categories)
	}
}
