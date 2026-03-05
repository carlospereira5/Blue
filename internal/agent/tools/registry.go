package tools

import "aria/internal/agent/llm"

// AriaTools retorna las definiciones de herramientas que el LLM puede invocar.
func AriaTools() []llm.ToolDef {
	return []llm.ToolDef{
		salesTool(),
		topProductsTool(),
		shiftExpensesTool(),
		supplierPaymentsTool(),
		stockTool(),
	}
}

func salesTool() llm.ToolDef {
	return llm.ToolDef{
		Name:        "get_sales",
		Description: "Obtiene el total de ventas en un rango de fechas, desglosado por método de pago (efectivo, tarjeta, etc.). NO usar para ranking de productos ni análisis de categorías.",
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio en formato YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin en formato YYYY-MM-DD"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func topProductsTool() llm.ToolDef {
	return llm.ToolDef{
		Name:        "get_top_products",
		Description: "Obtiene el ranking de productos más o menos vendidos en un rango de fechas. Usar sort_order 'asc' para ver productos con pocas o cero ventas (dead stock). NO usar para totales de dinero — usar get_sales para eso.",
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio en formato YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin en formato YYYY-MM-DD"},
			{Name: "category", Type: "string", Description: "Nombre de la categoría para filtrar (opcional)"},
			{Name: "limit", Type: "integer", Description: "Cantidad máxima de productos a retornar (default 10)"},
			{Name: "sort_order", Type: "string", Description: "Orden: 'desc' para más vendidos (default), 'asc' para menos vendidos o sin ventas", Enum: []string{"asc", "desc"}},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func shiftExpensesTool() llm.ToolDef {
	return llm.ToolDef{
		Name:        "get_shift_expenses",
		Description: "Obtiene todos los retiros de caja (PAY_OUT) por turno en un rango de fechas, incluyendo pagos a proveedores y gastos varios. Para desglose por proveedor específico usar get_supplier_payments.",
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio en formato YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin en formato YYYY-MM-DD"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func supplierPaymentsTool() llm.ToolDef {
	return llm.ToolDef{
		Name:        "get_supplier_payments",
		Description: "Obtiene los pagos a proveedores en un rango de fechas, agrupados por proveedor mediante alias. Incluye gastos sin clasificar. NO usar para gastos generales — usar get_shift_expenses para ver todos los retiros.",
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio en formato YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin en formato YYYY-MM-DD"},
			{Name: "supplier_name", Type: "string", Description: "Nombre del proveedor para filtrar (opcional)"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func stockTool() llm.ToolDef {
	return llm.ToolDef{
		Name:        "get_stock",
		Description: "Obtiene los niveles de stock actuales del inventario, opcionalmente filtrado por categoría. Los datos reflejan el último sync con Loyverse.",
		Parameters: []llm.ParamDef{
			{Name: "category", Type: "string", Description: "Nombre de la categoría para filtrar (opcional)"},
		},
	}
}
