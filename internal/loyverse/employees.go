package loyverse

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// GetEmployees obtiene una página de empleados.
func (c *Client) GetEmployees(ctx context.Context, limit int, cursor string) (*EmployeesResponse, error) {
	params := url.Values{
		"limit": {fmt.Sprintf("%d", clampLimit(limit))},
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	req, err := c.buildRequest(ctx, http.MethodGet, "/employees", params)
	if err != nil {
		return nil, err
	}
	var response EmployeesResponse
	return &response, c.do(req, &response)
}

// GetEmployeeByID obtiene un empleado específico por su ID.
func (c *Client) GetEmployeeByID(ctx context.Context, id string) (*Employee, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, "/employees/"+id, nil)
	if err != nil {
		return nil, err
	}
	var response Employee
	return &response, c.do(req, &response)
}

// GetAllEmployees obtiene TODOS los empleados, manejando la paginación automáticamente.
func (c *Client) GetAllEmployees(ctx context.Context) ([]Employee, error) {
	var all []Employee
	cursor := ""
	for {
		resp, err := c.GetEmployees(ctx, 250, cursor)
		if err != nil {
			return nil, fmt.Errorf("GetAllEmployees (cursor=%q): %w", cursor, err)
		}
		all = append(all, resp.Employees...)
		if resp.Cursor == "" {
			break
		}
		cursor = resp.Cursor
	}
	return all, nil
}
