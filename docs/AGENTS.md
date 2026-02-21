# Kiosco OS - Documentación del Proyecto

## Visión del Proyecto

Sistema de gestión integral para un kiosco que opera con Loyverse POS. El objetivo es unificar datos dispersos (transacciones, inventario, contabilidad, deudas) en un flujo de trabajo ordenado que genere visibilidad y predicibilidad financiera.

---

## Stack Tecnológico

| Capa | Tecnología | Justificación |
|------|------------|---------------|
| **Backend** | Go 1.21+ | Lenguaje del proyecto, alto rendimiento, binario único |
| **API** | Gin | Router HTTP minimalista y rápido para Go |
| **DB** | PostgreSQL | Volumen alto de transacciones (~500 tickets/día historico), ACID |
| **Hot Reload** | Air | Desarrollo con reload automático en Go |
| **Frontend** | React + Vite | Desarrollo rápido, компоненты reutilizables, ecosystem maduro |
| **Estilos** | Tailwind CSS 4 | Utilidades CSS, desarrollo rápido de UI |
| **Secretos** | Infisical | Manejo de API keys y variables sensibles |
| **Automatización** | Task (Taskfile) | Tasks para tests, builds, migrations |
| **CLI** | Charm CLI (Bubble Tea) | CLI interactiva para administración |

---

## Principios de Desarrollo

### Clean Architecture

```
├── cmd/                 # Entry points de la aplicación
├── internal/            # Código privado del proyecto
│   ├── api/             # Handlers HTTP, router, middleware
│   ├── service/         # Lógica de negocio
│   ├── repository/      # Acceso a datos
│   ├── model/           # Entidades y tipos del dominio
│   └── config/         # Configuración de la aplicación
├── pkg/                 # Código reutilizable (librerías internas)
├── web/                 # Frontend React
├── migrations/          # Migraciones de DB
├── scripts/             # Scripts auxiliares
├── task/                # Taskfiles para automatización
└── tests/               # Tests de integración
```

**Regla**: Cada archivo debe ser lean. Si un archivo supera las ~200 líneas, considerar-split.

### Testing

- **Tests unitarios** desde el inicio para cada función
- **Table-driven tests** para casos múltiples
- **命名** claro: `nombre_test.go`
- Ejecutar tests con `task test`

### Documentación y Comentarios

- Cada función exported debe tener doc comments
- Comentarios inline para lógica no-obvia
- README.md en cada módulo explicando su propósito

### Persistencia de Sesiones (Checkpoint)

- **Obligatorio**: Crear/actualizar `checkpoint.md` al final de cada sesión de trabajo con IA
- **Estructura del checkpoint**:
  - Fecha de la sesión
  - Resumen de lo que se trabajó
  - Cambios realizados (archivos creados/modificados)
  - Pendientes / siguiente pasos
  - Bloqueos o decisiones que necesitan revisión
- **Propósito**: Mantener contexto entre sesiones, ver evolución del proyecto y qué falta por hacer

---

## Estructura de Módulos

### Módulos Principales

| Módulo | Responsabilidad |
|--------|-----------------|
| **Loyverse Client** | Consumo de API de Loyverse, parsing de responses |
| **Inventory** | Gestión de productos, proveedores, lotes (FIFO) |
| **Accounting** | Caja, deudas, pagos, flujo de caja |
| **Metrics** | Cálculos de rentabilidad, rotación, KPIs |
| **WhatsApp Bot** | Interfaz conversacional con LLM |
| **Sync Service** | Sincronización periódica Loyverse → DB |

---

## Integraciones

### Loyverse POS

- **API**: `https://api.loyverse.com/v1.0`
- **Auth**: OAuth / API Token
- **Endpoints principales**:
  - `GET /receipts` - Transacciones
  - `GET /items` - Catálogo de productos
  - `GET /inventory` - Stock actual
  - `GET /categories` - Categorías

### WhatsApp + LLM

- Bot conversacional para consultas rápidas
- Registro de pagos por voz/texto
- Insights instantáneos

---

## CLI (Charm CLI)

La CLI de Charm se usa como interfaz de administración del sistema. Permite probar el cliente de Loyverse, ordenar respuestas, y administrar la app.

### Herramientas de Charm

| Herramienta | Uso |
|-------------|-----|
| **Bubble Tea** | Framework para crear CLIs interactivas en Go |
| **Woodpecker** | Frames UI en terminal |
| **Glow** | Render markdown en terminal |
| **Charm** | Utils varios para CLI |

### Instalación

```bash
# Instalar Charm
brew install charm/tap/charm

# Instalar Bubble Tea (para la CLI)
go get github.com/charmbracelet/bubbletea
```

---

## Convenciones de Código

### Go

- **Paquetes**: lowerCamelCase (`loyversecustomer`, no `LoyverseCustomer`)
- **Interfaces**: nombre terminar en `-er` (`Reader`, `Writer`, `LoyverseFetcher`)
- **Errores**: Wrapped con `fmt.Errorf("%w", err)`
- **Naming**: descriptivo, no abreviado (`GetSalesByDateRange`, no `GetSales`)

### Git

- Commits convencionales: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`
- No commits de binaries o secrets

### Tests

```go
// Estructura de test table-driven
func TestCalculateMargin(t *testing.T) {
    tests := []struct {
        name     string
        cost     float64
        price    float64
        expected float64
    }{
        {"basic margin", 100, 150, 50},
        {"zero cost", 0, 100, 100},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

---

## Commands Útiles

| Task | Descripción |
|------|-------------|
| `task dev` | Inicia el servidor con Air |
| `task test` | Ejecuta todos los tests |
| `task test:watch` | Tests en modo watch |
| `task db:migrate` | Ejecuta migraciones |
| `task db:seed` | Poblaje inicial de datos |
| `task lint` | Ejecuta linters |
| `task build` | Build de producción |

---

## Secrets (Infisical)

Variables necesarias (configurar en Infisical):

```env
# Loyverse
LOYVERSE_API_TOKEN=...

# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=kiosko
POSTGRES_PASSWORD=...
POSTGRES_DB=kiosko_os

# App
PORT=8080
ENV=development
```

---

## Flujo de Trabajo Diario

1. **Sincronización**: Fetch de receipts de Loyverse → Parse → Guardar en DB
2. **Contabilidad**: Registro de caja (apertura/cierre) → Cálculo de flujo
3. **Métricas**: Dashboard actualizado con ventas, márgenes, rotación
4. **WhatsApp**: Consultas rápidas, registro de pagos

---

## Referencias

- [Loyverse API Docs](https://developer.loyverse.com/docs/)
- [Go Clean Architecture](https://github.com/bxcodec/go-clean-architecture)
- [Gin Web Framework](https://gin-gonic.com/)
- [React + Vite](https://vitejs.dev/)
- [Tailwind CSS](https://tailwindcss.com/)
