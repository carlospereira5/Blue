package tools

import "aria/internal/agent/llm"

// AriaTools retorna las definiciones de herramientas que el LLM puede invocar.
// Las descripciones incluyen use cases positivos y anti-cases para guiar
// la selección correcta de herramientas por parte del LLM.
func AriaTools() []llm.ToolDef {
	return []llm.ToolDef{
		// Discovery (Nivel 1)
		getCategoriesToolDef(),
		getItemsToolDef(),
		// Analytics (Nivel 3)
		salesTool(),
		topProductsTool(),
		shiftExpensesTool(),
		supplierPaymentsTool(),
		stockTool(),
		salesVelocityTool(),
		cashFlowTool(),
		// Search (Nivel 2)
		searchProductTool(),
		searchCategoryTool(),
		searchEmployeeTool(),
		// Actions (Nivel 4)
		saveAliasTool(),
		saveMemoryTool(),
	}
}

func getCategoriesToolDef() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_categories",
		Description: `Lista las categorías de productos del catálogo. Usá esto para saber qué categorías existen antes de filtrar otras tools por categoría.

CUÁNDO USAR:
- Cuando el usuario pregunta qué categorías hay o quiere explorarlas.
- Antes de usar el parámetro "category" en get_top_products, get_stock o get_sales_velocity, si no conocés el nombre exacto.
- Para generar aliases de categorías o entender la estructura del catálogo.
Ejemplos: "¿qué categorías tenés?", "listar categorías", "¿cuál es el nombre exacto de la categoría de cigarrillos?"

CUÁNDO NO USAR:
- Si ya sabés el nombre exacto de la categoría que necesitás.
- Para buscar categorías por nombre coloquial con scoring → usar search_category.

Retorna: lista de categorías con id y nombre, ordenadas alfabéticamente.`,
		Parameters: []llm.ParamDef{
			{Name: "name", Type: "string", Description: "Filtro opcional por nombre (substring, case-insensitive). Sin filtro devuelve todas."},
		},
	}
}

func getItemsToolDef() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_items",
		Description: `Lista productos del catálogo con sus precios. Permite explorar qué productos existen, filtrar por categoría o buscar por nombre.

CUÁNDO USAR:
- Cuando el usuario pregunta qué productos hay en una categoría.
- Cuando pregunta el precio de un producto.
- Para explorar el catálogo antes de generar aliases o hacer análisis.
Ejemplos: "¿qué productos hay en Bebidas?", "¿cuánto cuesta la Coca-Cola?", "mostrarme los cigarrillos disponibles".

CUÁNDO NO USAR:
- Para saber cuánto se VENDIÓ de un producto → usar get_top_products.
- Para desambiguar un nombre coloquial con scoring → usar search_product.
- Para ver el stock disponible → usar get_stock.

Retorna: lista de productos con id, nombre, categoría y precio. Excluye productos archivados.`,
		Parameters: []llm.ParamDef{
			{Name: "category", Type: "string", Description: "Filtro por nombre exacto de categoría (case-insensitive). Obtené el nombre exacto con get_categories si no lo sabés."},
			{Name: "query", Type: "string", Description: "Filtro por nombre de producto (substring, case-insensitive)."},
			{Name: "limit", Type: "integer", Description: "Máximo de productos a retornar (default: 20, máximo: 100)."},
		},
	}
}

func salesTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_sales",
		Description: `Obtiene el TOTAL MONETARIO de ventas en un rango de fechas, desglosado por método de pago.

CUÁNDO USAR: cuando el usuario pregunta por dinero total, ingresos, ventas brutas/netas, o cuánto entró en efectivo/tarjeta.
Ejemplos: "¿cuánto se vendió hoy?", "¿cuánto entró en efectivo?", "ventas de la semana".

CUÁNDO NO USAR:
- Para saber cuántas unidades de un producto se vendieron → usar get_top_products.
- Para saber cuánto se gastó o retiró de caja → usar get_shift_expenses.
- Para saber cuánto se le pagó a un proveedor → usar get_supplier_payments.
- Para el balance neto (ventas menos gastos) → usar get_cash_flow.

Retorna: ventas brutas, netas, reembolsos, cantidad de transacciones, y desglose por método de pago (efectivo, tarjeta, etc.).`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin YYYY-MM-DD"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func topProductsTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_top_products",
		Description: `Obtiene el ranking de productos por cantidad vendida en un rango de fechas.

CUÁNDO USAR: cuando el usuario pregunta por productos específicos, rankings, qué se vende más o menos, o cuántas unidades de algo se vendieron.
Ejemplos: "¿qué es lo que más se vende?", "¿cuántas coca cola se vendieron?", "productos que no se mueven".

CUÁNDO NO USAR:
- Para saber el total de dinero vendido → usar get_sales.
- Para saber cuánto stock queda de un producto → usar get_stock.
- Para saber con qué urgencia reponer un producto → usar get_sales_velocity.

Parámetros clave:
- sort_order="desc" (default): productos MÁS vendidos primero.
- sort_order="asc": productos MENOS vendidos o con CERO ventas (dead stock). Usar para "qué no se vende".
- category: filtra por nombre de categoría (ej: "Bebidas", "Cigarrillos").`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin YYYY-MM-DD"},
			{Name: "category", Type: "string", Description: "Nombre exacto de la categoría para filtrar (opcional)"},
			{Name: "limit", Type: "integer", Description: "Máximo de productos a retornar (default: 10)"},
			{Name: "sort_order", Type: "string", Description: "'desc' = más vendidos (default), 'asc' = menos vendidos o sin ventas", Enum: []string{"asc", "desc"}},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func shiftExpensesTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_shift_expenses",
		Description: `Obtiene todos los RETIROS DE CAJA (PAY_OUT) por turno en un rango de fechas. Incluye pagos a proveedores, gastos operativos, y cualquier dinero que salió de la caja.

CUÁNDO USAR: cuando el usuario pregunta por gastos, retiros de caja, cuánto se gastó, o qué salió de la caja.
Ejemplos: "¿cuánto se gastó hoy?", "¿qué retiros hubo?", "gastos de la semana".

CUÁNDO NO USAR:
- Para saber cuánto se le pagó a un proveedor ESPECÍFICO → usar get_supplier_payments (agrupa por proveedor).
- Para saber cuánto se vendió → usar get_sales.
- Para el balance neto (ventas menos gastos) → usar get_cash_flow.

Retorna: lista de retiros agrupados por turno, con comentario (qué se pagó), monto, y fecha de cada gasto.`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin YYYY-MM-DD"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func supplierPaymentsTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_supplier_payments",
		Description: `Obtiene pagos a PROVEEDORES en un rango de fechas, agrupados por proveedor. Funciona analizando los comentarios de los retiros de caja y clasificándolos por proveedor usando aliases conocidos.

CUÁNDO USAR: cuando el usuario pregunta cuánto se le pagó a un proveedor, pagos a proveedores, o cuánto se compró.
Ejemplos: "¿cuánto le pagamos a Coca-Cola?", "pagos a proveedores", "¿cuánto se compró esta semana?".

CUÁNDO NO USAR:
- Para ver TODOS los gastos (no solo proveedores) → usar get_shift_expenses.
- Para saber cuánto se vendió → usar get_sales.

Retorna: pagos agrupados por proveedor, total general, y gastos "sin clasificar" (retiros que no matchearon con ningún proveedor conocido).`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin YYYY-MM-DD"},
			{Name: "supplier_name", Type: "string", Description: "Nombre del proveedor para filtrar (opcional)"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func stockTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_stock",
		Description: `Obtiene los niveles de STOCK ACTUAL del inventario. Refleja el último sync con Loyverse (puede tener hasta 2 minutos de delay).

CUÁNDO USAR: cuando el usuario pregunta por inventario, cuánto queda, stock, o productos disponibles.
Ejemplos: "¿cuánto stock queda de bebidas?", "¿qué hay en inventario?", "¿cuántas coca cola nos quedan?".

CUÁNDO NO USAR:
- Para saber qué productos se VENDEN más o menos → usar get_top_products.
- Para saber cuánto DINERO se vendió → usar get_sales.
- Para saber con qué URGENCIA reponer un producto → usar get_sales_velocity (combina stock + velocidad de venta).

Retorna: lista de productos con cantidad en stock, agrupados por categoría.`,
		Parameters: []llm.ParamDef{
			{Name: "category", Type: "string", Description: "Nombre exacto de la categoría para filtrar (opcional)"},
		},
	}
}

func salesVelocityTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_sales_velocity",
		Description: `Calcula la VELOCIDAD DE VENTA por producto (unidades/día) y los días de stock restante. Combina ventas históricas con el inventario actual.

CUÁNDO USAR: cuando el usuario pregunta por reposición, urgencia de compra, rotación de productos o dead stock.
Ejemplos: "¿qué necesitamos pedir?", "¿cuándo se agota la Coca-Cola?", "¿qué no se está moviendo?", "¿qué tiene poco stock?".

CUÁNDO NO USAR:
- Para saber cuántas unidades se vendieron (sin cruzar con stock) → usar get_top_products.
- Para saber cuánto stock hay sin importar la rotación → usar get_stock.

Retorna: lista ordenada por urgencia (menor días_de_stock primero). Dead stock (stock > 0, ventas = 0) al final. Incluye unidades/día, stock actual y días estimados hasta agotarse.`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio del período de análisis YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin del período de análisis YYYY-MM-DD"},
			{Name: "category", Type: "string", Description: "Nombre exacto de la categoría para filtrar (opcional)"},
			{Name: "limit", Type: "integer", Description: "Máximo de productos a retornar (default: 10)"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func searchProductTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "search_product",
		Description: `Busca un producto por nombre usando fuzzy matching. Retorna los mejores candidatos con score de confianza.

CUÁNDO USAR: cuando el usuario menciona un producto por nombre coloquial o abreviado y necesitás resolver el nombre canónico antes de llamar otra tool.
Ejemplos: "palmal azul", "coca", "sprite grande".

Retorna: lista de hasta 5 candidatos con id, nombre canónico y confianza (1.0=exacto, 0.9=prefijo, 0.7=contiene).
- Si confianza ≥ 0.9 y hay un solo resultado: usarlo directamente.
- Si hay múltiples candidatos o confianza < 0.9: presentar opciones al usuario para confirmar.
- Después de confirmación del usuario: llamar save_alias con el resultado confirmado.`,
		Parameters: []llm.ParamDef{
			{Name: "query", Type: "string", Description: "Nombre o alias del producto a buscar"},
		},
		Required: []string{"query"},
	}
}

func searchCategoryTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "search_category",
		Description: `Busca una categoría por nombre usando fuzzy matching. Útil para resolver aliases coloquiales antes de filtrar otras tools.

CUÁNDO USAR: cuando el usuario menciona una categoría con nombre coloquial (ej: "cigarros", "bebidas", "golosinas") y necesitás el nombre exacto para pasarlo como parámetro category a otra tool.

Retorna: candidatos con id, nombre canónico y confianza.
- Si confianza ≥ 0.9 y un solo resultado: usar ese nombre en el parámetro category de la tool siguiente.
- Si hay ambigüedad: preguntar al usuario.
- Después de confirmación: llamar save_alias.`,
		Parameters: []llm.ParamDef{
			{Name: "query", Type: "string", Description: "Nombre o alias de la categoría a buscar"},
		},
		Required: []string{"query"},
	}
}

func searchEmployeeTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "search_employee",
		Description: `Busca un empleado por nombre usando fuzzy matching.

CUÁNDO USAR: cuando el usuario menciona a un empleado por apodo o nombre parcial.
Ejemplos: "carlos", "mari", "el nuevo".

Retorna: candidatos con id, nombre canónico y confianza.`,
		Parameters: []llm.ParamDef{
			{Name: "query", Type: "string", Description: "Nombre o alias del empleado a buscar"},
		},
		Required: []string{"query"},
	}
}

func saveAliasTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "save_alias",
		Description: `Guarda un alias para una entidad después de que el usuario confirmó la desambiguación. Permite que futuras búsquedas de ese alias resuelvan directamente sin fuzzy search.

CUÁNDO USAR: solo después de que el usuario confirmó explícitamente a qué entidad se refería (tras una desambiguación). NUNCA llamar sin confirmación del usuario.

entity_type válidos: "product", "category", "employee".`,
		Parameters: []llm.ParamDef{
			{Name: "entity_type", Type: "string", Description: "Tipo de entidad: 'product', 'category' o 'employee'"},
			{Name: "entity_id", Type: "string", Description: "ID de la entidad en Loyverse (obtenido del resultado de search_*)"},
			{Name: "canonical_name", Type: "string", Description: "Nombre canónico de la entidad (tal como está en el sistema)"},
			{Name: "alias", Type: "string", Description: "El término que usó el usuario y que debe mapearse a esta entidad"},
		},
		Required: []string{"entity_type", "entity_id", "canonical_name", "alias"},
	}
}

func saveMemoryTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "save_memory",
		Description: `Guarda una memoria sobre el usuario actual para personalizar futuras conversaciones.

CUÁNDO USAR: cuando el usuario revela algo relevante sobre sus preferencias, hábitos o contexto que debería influir en futuras respuestas.
Ejemplos:
- El usuario pide que siempre muestres los datos en resumen sin detalle
- El usuario confirma que "mamá" y "la dueña" son la misma persona
- El usuario explica un flujo de trabajo recurrente ("los martes siempre pregunto por las ventas de la semana")
- El usuario prefiere ver los montos en miles (ej: "150K" en lugar de "$150.000")

CUÁNDO NO USAR:
- Para aliases de productos, categorías o empleados → usar save_alias
- Para datos transaccionales (ventas, gastos, stock) → usar las tools de negocio
- Para información ya sabida de conversaciones anteriores (ya está en memorias)
- Información efímera o de una sola vez que no aplica al futuro

Guardá memorias concretas y accionables, en una oración. Evitá redundancias con memorias existentes.`,
		Parameters: []llm.ParamDef{
			{Name: "content", Type: "string", Description: "La memoria a guardar: una oración clara y accionable sobre el usuario"},
		},
		Required: []string{"content"},
	}
}

func cashFlowTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_cash_flow",
		Description: `Calcula el FLUJO DE CAJA del período: ventas netas (ingresos) menos egresos de caja (PAY_OUT: gastos, proveedores) más entradas extra (PAY_IN). Es el balance operativo real.

CUÁNDO USAR: cuando el usuario pregunta por el balance del día/semana, cuánto entró y salió, cuánto quedó en caja, o el flujo neto.
Ejemplos: "¿cuánto entró y salió hoy?", "¿cuál es el balance del día?", "¿cuánto quedó en caja?", "¿cómo estuvo la semana financieramente?".

CUÁNDO NO USAR:
- Para ver solo las ventas (sin egresos) → usar get_sales.
- Para ver el detalle de cada retiro de caja → usar get_shift_expenses.
- Para ver cuánto se le pagó a un proveedor → usar get_supplier_payments.

Retorna: ventas_netas, egresos_caja, entradas_caja, flujo_neto, periodo_dias.`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin YYYY-MM-DD"},
		},
		Required: []string{"start_date", "end_date"},
	}
}
