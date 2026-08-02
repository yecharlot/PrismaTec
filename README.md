# PrismaTec / Alset (P.TEC-AN v4.0)

Red descentralizada híbrida escrita en Go. Combina libp2p, un motor Lisp embebido, IA distribuida (spikes / sinapsis / memoria), sistema de pulsos SSE y un panel de administración.

## Estado actual del repositorio

Este repositorio ha sido **reestructurado** para dejar de ser un monolito puro y preparar la persistencia en Supabase.

```
PrismaTec/
├── cmd/prisma-tec/          # Punto de entrada (main)
├── internal/
│   ├── node/                # Núcleo del nodo (NodoAlset + lógica principal)
│   ├── persistence/         # Capa de persistencia (Local + Supabase ready)
│   ├── lisp/                # (próxima extracción)
│   ├── neural/              # (próxima extracción)
│   ├── pulse/               # (próxima extracción)
│   ├── p2p/                 # (próxima extracción)
│   ├── agents/              # (próxima extracción)
│   ├── blocks/              # (próxima extracción)
│   ├── sync/                # (próxima extracción)
│   └── httpapi/             # (próxima extracción)
├── docs/
│   └── ARCHITECTURE.md
├── static/
├── alset_data/              # Persistencia local por defecto
├── Dockerfile
├── go.mod
└── README.md
```

> **Nota**: La mayor parte de la lógica todavía vive en `internal/node/node.go` (el antiguo `main.go`). Las extracciones a paquetes más pequeños se harán de forma incremental en siguientes PRs para no romper funcionalidad.

## Características principales

- **Nodo Local** vs **Nodo Relay (Render)**  
  Detectado automáticamente con la variable de entorno `RENDER`.
- Motor **Lisp** embebido.
- IA distribuida: spikes, pesos sinápticos Hebbianos, inferencia y memoria distribuida.
- Sistema de **pulsos SSE** (`/api/pulse`) para comunicación resiliente.
- libp2p (Host + Kademlia DHT + GossipSub).
- Bloques con CIDs (IPFS-style).
- Panel de administración y apps estáticas.

## Persistencia

Se eliminó por completo la persistencia hacia GitHub.

Ahora existe una capa abstracta (`internal/persistence`):

| Backend   | Activación                                      | Descripción                     |
|-----------|--------------------------------------------------|---------------------------------|
| **Local** | Por defecto                                      | Archivos en `alset_data/`       |
| **Supabase** | `SUPABASE_URL` + `SUPABASE_SERVICE_KEY`        | Tabla `alset_kv` (key/value)    |

### Preparación de Supabase (siguiente paso)

1. Crea un proyecto en Supabase.
2. Ejecuta este SQL:

```sql
create table if not exists alset_kv (
  key        text primary key,
  value      jsonb not null,
  updated_at timestamptz default now()
);
```

3. Configura las variables de entorno (en Render o local):

```bash
export SUPABASE_URL="https://xxxx.supabase.co"
export SUPABASE_SERVICE_KEY="eyJhbGciOiJI..."
# opcional:
export SUPABASE_TABLE="alset_kv"
```

4. El nodo detectará automáticamente Supabase y lo usará.

## Cómo ejecutar

### Local

```bash
go run ./cmd/prisma-tec
# o
go build -o prisma-tec ./cmd/prisma-tec && ./prisma-tec
```

### Render

El `Dockerfile` ya está preparado. La variable `RENDER` se inyecta automáticamente.

## Variables de entorno relevantes

| Variable                 | Descripción                                      |
|--------------------------|--------------------------------------------------|
| `RENDER`                 | Si está presente → modo Relay (solo servidor de pulsos) |
| `SUPABASE_URL`           | URL del proyecto Supabase                        |
| `SUPABASE_SERVICE_KEY`   | Service role key                                 |
| `SUPABASE_TABLE`         | Nombre de la tabla (default `alset_kv`)          |

## Seguridad

- **Nunca** subas tokens o service keys al repositorio.
- El antiguo `github_config.json` ha sido eliminado del flujo.
- Usa variables de entorno o un secret manager.

## Roadmap de modularización

1. ✅ Estructura de carpetas + capa de persistencia + documentación
2. ⏳ Extracción completa del motor Lisp
3. ⏳ Extracción del sistema de pulsos
4. ⏳ Extracción de la capa neural
5. ⏳ Extracción de handlers HTTP
6. ⏳ Tests unitarios e de integración

## Licencia

Por definir por el autor del repositorio.
