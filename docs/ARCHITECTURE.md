# Arquitectura de PrismaTec / Alset (P.TEC-AN v4.0)

## Persistencia (Supabase)

Proyecto Supabase: **Alset** (`uysvbxawytsegxcufdds`)

| Tabla | Uso |
|-------|-----|
| `alset_agents` | Un registro por agente (`id`, `data` jsonb) |
| `alset_blocks` | Un registro por CID (`cid`, `data` bytea/base64, `size`) |
| `alset_neural_state` | Estado neuronal (`id` default `main`, `state` jsonb) |
| `alset_kv` | Key/value genérico (nombres DNS, backup de estado) |

Variables de entorno:

```
SUPABASE_URL=https://uysvbxawytsegxcufdds.supabase.co
SUPABASE_SERVICE_KEY=<secret>
```

## Estructura del repositorio

```
cmd/prisma-tec/          → entrypoint
internal/
  node/                  → núcleo (aún denso; extracción incremental)
  persistence/           → Store, Local, Supabase (tablas estructuradas)
  lisp/ neural/ pulse/   → (próximas extracciones)
  agents/ blocks/ p2p/ sync/ httpapi/ config/
docs/
static/
```

## Flujo de persistencia

1. **Arranque** → `CargarEstado()`  
   - agentes desde `alset_agents`  
   - blocks desde `alset_blocks`  
   - neural desde `alset_neural_state`  
   - nombres desde `alset_kv`

2. **Durante uso / shutdown** → `PersistirLocamente()`  
   - escribe en las cuatro tablas anteriores

## Local vs Relay

Detectado con `RENDER`. En Render solo actúa como servidor de pulsos SSE.
