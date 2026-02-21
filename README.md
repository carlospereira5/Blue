# Kiosco OS

## El Problema

Tenemos un kiosco con buen flujo de clientes (~500 ventas/día), pero hay un caos operativo enorme:

### Problemas Detectados

1. **Contabilidad ciega**
   - No se hace caja diariamente
   - No se tiene noción de costos de operación
   - No se saben los márgenes de beneficio por producto
   - No hay registro de entradas/salidas de dinero

2. **Inventario sin control**
   - No se know el precio de costo de los productos
   - No se sabe qué productos generan más ganancia
   - No hay métricas de rotación (qué se vende rápido, qué no)
   - Un solo proveedor por producto (sin opción de comparar precios)

3. **Deudas descontroladas**
   - Deuda con BAT (Chile Tabacos) - deuda fija semanal
   - Deuda con Ingrid (tía) - compras a su nombre
   - Todo registrado en conversaciones de WhatsApp, nada centralizado
   - Sin plan de pagos, sin proyecciones

4. **Baja de ventas**
   - Las ventas bajaron drásticamente en los últimos meses
   - Factor socioeconómico, no controlable directamente
   - Sin datos no se puede optimizar qué productos vender

### La Cuestión de Fondo

> No es que el negocio deje de vender. El problema es que no sabemos **qué** vender para maximizar el flujo de efectivo con las ventas que ya tenemos.

Si no podemos hacer que venga más gente, podemos decidir **qué** le vendemos a esa gente.

---

## Por Qué Ocurre Esto

El negocio opera con **intuición**, no con **datos**:

- Usamos Loyverse POS para vender (y funciona bien)
- Loyverse genera un registro histórico enorme de transacciones
- **Esa data nunca se usa** para tomar decisiones

Loyverse es una herramienta de venta, no de gestión. Nos da el qué pasó, pero no el qué hacer con esa información.

---

## La Solución

### Idea Central

> No reemplazar Loyverse. **Complementarlo**.

Loyverse ya tiene todas las transacciones digitalizadas. El problema es que esa información está aislada y sin procesar.

### Arquitectura de la Solución

```
┌─────────────────────────────────────────────────────────────────────┐
│                        KIOSCO OS                                     │
│                                                                      │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐                │
│  │  LOYVERSE   │   │  INVENTARIO │   │ CONTABILIDAD│                │
│  │   CLIENT    │   │  (múltiples│   │ (caja +     │                │
│  │  - receipts │   │   proveed.)│   │  deudas)    │                │
│  │  - items    │   │  - precio  │   │  - apertura │                │
│  │  - ventas   │   │    costo   │   │  - cierre   │                │
│  └──────┬──────┘   │  - FIFO    │   │  - pagos    │                │
│         │          └──────┬──────┘   └──────┬──────┘                │
│         └─────────────────┼─────────────────┘                        │
│                           ▼                                          │
│                    ┌─────────────┐                                    │
│                    │  DATABASE   │  ◄── Transacciones = Estado        │
│                    └─────────────┘       (matemática pura)           │
│                           │                                          │
│         ┌─────────────────┼─────────────────┐                        │
│         ▼                 ▼                 ▼                        │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐                 │
│  │   FRONTEND  │   │   WHATSAPP  │   │   MÉTRICAS  │                 │
│  │  (métricas) │   │  + LLM bot  │   │ (automático)│                 │
│  └─────────────┘   └─────────────┘   └─────────────┘                 │
└─────────────────────────────────────────────────────────────────────┘
```

### Módulos

| Módulo | Responsabilidad |
|--------|-----------------|
| **Loyverse Client** | Fetch de receipts, items, inventory desde la API de Loyverse |
| **Inventory** | Gestión de múltiples proveedores por producto + FIFO para costos |
| **Accounting** | Caja diaria, registro de deudas y pagos |
| **Metrics** | Márgenes por producto, rotación, flujo de caja |
| **WhatsApp Bot** | Interfaz conversacional (registrar pagos, consultar métricas) |
| **Frontend** | Dashboard con métricas visuales |

### Flujo de Datos

```
MAÑANA                         DÍA                          NOCHE
   │                              │                             │
   ▼                              ▼                             ▼
┌────────┐                  ┌──────────┐                 ┌───────────┐
│Apertura│  ── ventas ──>  │ Loyverse │  ── sync ──>   │ Dashboard │
│ caja   │                  │   POS    │                 │ actualizado│
└────────┘                  └──────────┘                 └───────────┘
   │                                                      │
   └─────────────────► REGISTRO ◄─────────────────────────┘
        (qué entró en caja)      (todo unificado)
```

---

## Decisiones Técnicas

### Por qué no reemplazar Loyverse

- Ya tiene 500 ventas/día digitalizadas
- Es bueno para lo que fue diseñado (facilitar la venta)
- API pública y bien documentada
- No tiene sentido reinventar lo que ya funciona

### Por qué Go + PostgreSQL

- **Go**: Alto rendimiento, binario único, manejo excelente de JSON (API de Loyverse), fácil de mantener
- **PostgreSQL**: Miles de transacciones históricas (500/día × años), necesitamos ACID y queries complejas para métricas
- **Gin**: Router HTTP minimalista y rápido
- **React + Vite + Tailwind**: Frontend rápido de desarrollar, buena experiencia para métricas

### Por qué WhatsApp como interfaz

- Ya está en el teléfono del cajero
- No requiere aprender una nueva app
- El bot puede recibir comandos como "registré pago de $50.000 a BAT"
- Consultas naturales: "¿cuánto vendí hoy?"

---

## ¿Qué Obtenemos?

### Week 1: Visibilidad
- [ ] Sync automático de receipts de Loyverse
- [ ] Dashboard con productos por margen y rotación
- [ ] Sabés qué productos te dejan plata

### Week 2: Control
- [ ] Registro de caja diario (apertura/cierre)
- [ ] Repo de deuda con Ingrid y BAT
- [ ] Métricas: venta diaria, diferencia de caja

### Week 3: Predictibilidad
- [ ] Proyección de flujo de caja
- [ ] Plan de pagos automatizado
- [ ] Alerts: "para pagar a BAT el viernes, necesitás vender $X hoy"

---

## La Matemática Base

> Estado actual = Estado inicial + Σ(transacciones históricas)

Esta ecuación simple es el fundamento de todo:

- **Inventario** = inventario inicial + compras - ventas
- **Caja** = caja inicial + ventas - gastos - pagos de deudas

Si automatizamos la extracción de receipts de Loyverse, **tenemos todo**. Solo falta unirlos y procesarlos.

---

## Estado del Proyecto

**En desarrollo**: Schema de DB y cliente de Loyverse en progreso.

---

## Referencias

- [Loyverse API](https://developer.loyverse.com/docs/)
- [Clean Architecture en Go](https://github.com/bxcodec/go-clean-architecture)
- [Gin Web Framework](https://gin-gonic.com/)
- [React + Vite](https://vitejs.dev/)
- [Tailwind CSS](https://tailwindcss.com/)
