package agent

// lumiTools retorna las herramientas que el LLM puede invocar para Lumi.
func lumiTools() []ToolDef {
	return []ToolDef{
		salesTool(),
		topProductsTool(),
		shiftExpensesTool(),
		supplierPaymentsTool(),
		stockTool(),
	}
}

func salesTool() ToolDef {
	return ToolDef{
		Name:        "get_sales",
		Description: "Obtiene el total de ventas en un rango de fechas, desglosado por método de pago (efectivo, tarjeta, etc.)",
		Parameters: []ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio en formato YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin en formato YYYY-MM-DD"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func topProductsTool() ToolDef {
	return ToolDef{
		Name:        "get_top_products",
		Description: "Obtiene los productos más vendidos (o menos vendidos) en un rango de fechas, opcionalmente filtrado por categoría",
		Parameters: []ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio en formato YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin en formato YYYY-MM-DD"},
			{Name: "category", Type: "string", Description: "Nombre de la categoría para filtrar (opcional)"},
			{Name: "limit", Type: "integer", Description: "Cantidad máxima de productos a retornar (default 10)"},
			{Name: "sort_order", Type: "string", Description: "Orden: 'desc' para más vendidos (default), 'asc' para menos vendidos", Enum: []string{"asc", "desc"}},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func shiftExpensesTool() ToolDef {
	return ToolDef{
		Name:        "get_shift_expenses",
		Description: "Obtiene los gastos (retiros de caja / pay outs) por turno en un rango de fechas",
		Parameters: []ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio en formato YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin en formato YYYY-MM-DD"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func supplierPaymentsTool() ToolDef {
	return ToolDef{
		Name:        "get_supplier_payments",
		Description: "Obtiene los pagos a proveedores en un rango de fechas, extraídos de los retiros de caja (pay outs). Opcionalmente filtra por un proveedor específico",
		Parameters: []ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio en formato YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin en formato YYYY-MM-DD"},
			{Name: "supplier_name", Type: "string", Description: "Nombre del proveedor para filtrar (opcional)"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func stockTool() ToolDef {
	return ToolDef{
		Name:        "get_stock",
		Description: "Obtiene los niveles de stock actuales, opcionalmente filtrado por categoría",
		Parameters: []ParamDef{
			{Name: "category", Type: "string", Description: "Nombre de la categoría para filtrar (opcional)"},
		},
	}
}
