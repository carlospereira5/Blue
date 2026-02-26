package agent

import "google.golang.org/genai"

// lumiTools retorna las herramientas que Gemini puede invocar para Lumi.
func lumiTools() []*genai.Tool {
	return []*genai.Tool{
		{FunctionDeclarations: []*genai.FunctionDeclaration{
			salesTool(),
			topProductsTool(),
			shiftExpensesTool(),
			supplierPaymentsTool(),
			stockTool(),
		}},
	}
}

func salesTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "get_sales",
		Description: "Obtiene el total de ventas en un rango de fechas, desglosado por método de pago (efectivo, tarjeta, etc.)",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"start_date": {
					Type:        genai.TypeString,
					Description: "Fecha de inicio en formato YYYY-MM-DD",
				},
				"end_date": {
					Type:        genai.TypeString,
					Description: "Fecha de fin en formato YYYY-MM-DD",
				},
			},
			Required: []string{"start_date", "end_date"},
		},
	}
}

func topProductsTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "get_top_products",
		Description: "Obtiene los productos más vendidos (o menos vendidos) en un rango de fechas, opcionalmente filtrado por categoría",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"start_date": {
					Type:        genai.TypeString,
					Description: "Fecha de inicio en formato YYYY-MM-DD",
				},
				"end_date": {
					Type:        genai.TypeString,
					Description: "Fecha de fin en formato YYYY-MM-DD",
				},
				"category": {
					Type:        genai.TypeString,
					Description: "Nombre de la categoría para filtrar (opcional)",
				},
				"limit": {
					Type:        genai.TypeInteger,
					Description: "Cantidad máxima de productos a retornar (default 10)",
				},
				"sort_order": {
					Type:        genai.TypeString,
					Description: "Orden: 'desc' para más vendidos (default), 'asc' para menos vendidos",
					Enum:        []string{"asc", "desc"},
				},
			},
			Required: []string{"start_date", "end_date"},
		},
	}
}

func shiftExpensesTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "get_shift_expenses",
		Description: "Obtiene los gastos (retiros de caja / pay outs) por turno en un rango de fechas",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"start_date": {
					Type:        genai.TypeString,
					Description: "Fecha de inicio en formato YYYY-MM-DD",
				},
				"end_date": {
					Type:        genai.TypeString,
					Description: "Fecha de fin en formato YYYY-MM-DD",
				},
			},
			Required: []string{"start_date", "end_date"},
		},
	}
}

func supplierPaymentsTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "get_supplier_payments",
		Description: "Obtiene los pagos a proveedores en un rango de fechas, extraídos de los retiros de caja (pay outs). Opcionalmente filtra por un proveedor específico",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"start_date": {
					Type:        genai.TypeString,
					Description: "Fecha de inicio en formato YYYY-MM-DD",
				},
				"end_date": {
					Type:        genai.TypeString,
					Description: "Fecha de fin en formato YYYY-MM-DD",
				},
				"supplier_name": {
					Type:        genai.TypeString,
					Description: "Nombre del proveedor para filtrar (opcional)",
				},
			},
			Required: []string{"start_date", "end_date"},
		},
	}
}

func stockTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "get_stock",
		Description: "Obtiene los niveles de stock actuales, opcionalmente filtrado por categoría",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"category": {
					Type:        genai.TypeString,
					Description: "Nombre de la categoría para filtrar (opcional)",
				},
			},
		},
	}
}
