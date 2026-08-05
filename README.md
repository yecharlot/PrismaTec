# Alset (P.TEC-AN v4.0)

Alset es una red peer-to-peer escrita en Go. Cada máquina que ejecuta el binario se convierte en un **nodo**: puede guardar datos, hablar con otros nodos, evaluar scripts Lisp y participar en una capa ligera de IA distribuida.

Hay dos formas de correr el mismo programa:

| Modo | Cuándo se activa | Qué hace |
|------|------------------|----------|
| **Local** | No existe la variable `RENDER` | Nodo completo: sirve la API, se conecta a la red y actúa como cliente de pulsos |
| **Relay** | Existe `RENDER` (típico en Render.com) | Solo actúa como servidor de pulsos y punto de sincronización |

No hace falta compilar dos binarios distintos. El comportamiento se decide al arrancar.

---

## Qué necesitas para empezar

- Go 1.26 o superior
- (Opcional) Un proyecto en [Supabase](https://supabase.com) si quieres guardar el estado en la nube
- (Opcional) Una cuenta en Render si vas a desplegar el relay

### Arranque en local

```bash
git clone https://github.com/yecharlot/PrismaTec.git
cd PrismaTec
go run ./cmd/prisma-tec
```

El panel de administración queda en:

```
http://localhost:8080/static/index.html
```

La API escucha en el puerto **8080** por defecto. Puedes pasar otro puerto como argumento:

```bash
go run ./cmd/prisma-tec 9090
```

### Arranque con Supabase

Define estas variables antes de lanzar el nodo:

```bash
export SUPABASE_URL="https://TU_PROYECTO.supabase.co"
export SUPABASE_SERVICE_KEY="tu_secret_key"
go run ./cmd/prisma-tec
```

Si no las defines, el nodo guarda todo en disco dentro de la carpeta `alset_data/`.

---

## Cómo está organizado el código

```
PrismaTec/
├── cmd/prisma-tec/     # Único punto de entrada
├── internal/           # Lógica interna (no se importa desde fuera)
├── docs/               # Documentación de arquitectura
├── static/             # Panel web y apps estáticas
├── alset_data/         # Datos locales (si no usas Supabase)
├── Dockerfile          # Imagen para Render u otros hosts
├── go.mod
└── README.md
```

### `cmd/prisma-tec`

Solo arranca el nodo. No contiene lógica de negocio. Si mañana quieres otro binario (por ejemplo una CLI de administración), se añade aquí sin tocar el resto.

### `internal/node`

El corazón del sistema. Aquí vive `NodoAlset`: conexión a la red (libp2p), sincronización, arranque del servidor HTTP y del sistema de pulsos.

Archivos relevantes:

| Archivo | Responsabilidad |
|---------|-----------------|
| `node.go` | Ciclo de vida del nodo, P2P, sincronización, persistencia |
| `http.go` | Rutas HTTP y panel |
| `pulse.go` | Clientes y servidor SSE de pulsos |
| `host_adapter.go` | Adapta el nodo a la interfaz que usa Lisp |
| `types.go` | Tipos propios del nodo y alias a otros paquetes |
| `helpers.go` | Utilidades pequeñas (UUID, JSON canónico) |

### `internal/lisp`

Motor Lisp embebido. No conoce los detalles internos del nodo: habla con él a través de una interfaz (`nodeiface.Host`). Así se puede cambiar el nodo sin reescribir el intérprete.

### `internal/nodeiface`

Contrato que el motor Lisp (y en el futuro otros módulos) usa para pedir cosas al nodo: crear agentes, firmar, publicar en la red, etc.

### `internal/agents`

Modelos de agentes, módulos, entidades y un registro compartido (`agents.Global`) para no repartir mapas sueltos por todo el código.

### `internal/neural`

Tipos de la capa de IA distribuida: estado de neurona, pesos sinápticos, peticiones de inferencia y consultas de memoria.

### `internal/poh`

Proof of Humanity: eventos y pruebas de sesión. El estado vive en `poh.Global`.

### `internal/persistence`

Cómo se guarda y se carga el estado.

| Backend | Cuándo se usa |
|---------|----------------|
| **Supabase** | Si están `SUPABASE_URL` y `SUPABASE_SERVICE_KEY` |
| **Disco local** | En cualquier otro caso (`alset_data/`) |

Tablas en Supabase:

| Tabla | Contenido |
|-------|-----------|
| `alset_agents` | Un registro por agente |
| `alset_blocks` | Un registro por bloque/CID |
| `alset_neural_state` | Estado neuronal |
| `alset_kv` | Nombres DNS y datos genéricos en formato clave/valor |

### `internal/pulse`

Tipos del sistema de pulsos (SSE). La lógica de conexión sigue en `node/pulse.go`; este paquete concentra las estructuras para poder reutilizarlas.

### Carpetas aún vacías (`blocks`, `config`, `httpapi`, `p2p`, `sync`)

Reservadas para seguir separando responsabilidades sin romper lo que ya funciona. No hace falta tocarlas para usar el nodo hoy.

---

## Operaciones habituales por API

### Crear un agente

```bash
curl -X POST https://TU_HOST/api/crear-agente
```

Respuesta típica:

```json
{
  "id": "cc356735b1e69431",
  "root_cid": "",
  "balance_utxo": 0,
  "ultima_actualizacion": 1785695861
}
```

### Listar agentes

```bash
curl https://TU_HOST/api/agentes/
```

### Listar bloques

```bash
curl https://TU_HOST/api/ipfs/list
```

### Ver peers de la red

```bash
curl https://TU_HOST/api/network/peers
```

En local sustituye `TU_HOST` por `http://localhost:8080`.

---

## Persistencia: cuándo se lee y se escribe

1. **Al arrancar** el nodo carga agentes, nombres, bloques y estado neural (desde Supabase o desde disco).
2. **Durante el uso** se guarda cuando creas o modificas agentes, registras DNS, publicas bloques o ejecutas ciertas funciones Lisp.
3. **Al apagar** (Ctrl+C o reinicio del servicio) se hace un guardado final.

Si usas Supabase, después de crear un agente deberías verlo en la tabla `alset_agents`.

---

## Despliegue en Render

1. Conecta este repositorio al servicio web de Render.
2. Añade las variables de entorno:
   - `SUPABASE_URL`
   - `SUPABASE_SERVICE_KEY`
3. Render inyecta `RENDER` solo; con eso el nodo entra en modo relay.
4. El `Dockerfile` del repo ya construye desde `./cmd/prisma-tec`.

Tras un deploy limpio, en los logs debe aparecer algo como:

```text
✅ Persistencia: Supabase → https://….supabase.co
🟢 Nodo ejecutándose en Render (servidor de pulsos)
```

Si ves mensajes de `GITHUB_TOKEN` o `github_config.json`, el servicio todavía está corriendo una versión antigua: fuerza un deploy con caché limpia.

---

## Variables de entorno

| Variable | Obligatoria | Descripción |
|----------|-------------|-------------|
| `RENDER` | No | Si existe, el nodo opera en modo relay |
| `SUPABASE_URL` | No* | URL del proyecto Supabase |
| `SUPABASE_SERVICE_KEY` | No* | Clave secret/service role |
| `SUPABASE_TABLE` | No | Tabla KV (por defecto `alset_kv`) |

\*Obligatorias solo si quieres persistencia en Supabase.

No subas claves al repositorio. Usa secretos del proveedor (Render, etc.).

---

## SQL inicial en Supabase

Si el proyecto es nuevo, ejecuta esto en el SQL Editor:

```sql
create table if not exists alset_kv (
  key        text primary key,
  value      jsonb not null,
  updated_at timestamptz not null default now()
);

create table if not exists alset_agents (
  id         text primary key,
  data       jsonb not null,
  updated_at timestamptz not null default now()
);

create table if not exists alset_blocks (
  cid        text primary key,
  data       bytea,
  size       integer,
  created_at timestamptz not null default now()
);

create table if not exists alset_neural_state (
  id         text primary key default 'main',
  state      jsonb not null,
  updated_at timestamptz not null default now()
);

create index if not exists idx_alset_kv_updated on alset_kv (updated_at desc);
```

---

## Compilar para producción

```bash
go build -o prisma-tec ./cmd/prisma-tec
./prisma-tec
```

O con Docker:

```bash
docker build -t alset .
docker run -p 8080:8080 \
  -e SUPABASE_URL=... \
  -e SUPABASE_SERVICE_KEY=... \
  alset
```

---

## Dónde seguir leyendo

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — mapa de componentes y flujo de datos
- Panel en runtime: `/static/index.html`

---

## Licencia

Por definir por el autor del repositorio.
