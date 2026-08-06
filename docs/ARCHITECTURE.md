Documentación de uso orientada a personas: [GUIA.md](GUIA.md).

# Arquitectura de Alset

Este documento describe cómo encajan las piezas del sistema. Está pensado para quien va a tocar el código o a operar un nodo en producción.

## Idea general

Un nodo Alset es un proceso Go que:

1. Se une a una red libp2p (DHT + GossipSub).
2. Expone una API HTTP y un panel web.
3. Puede evaluar scripts Lisp.
4. Mantiene un estado (agentes, bloques, nombres, capa neural).
5. Guarda ese estado en disco o en Supabase.
6. Intercambia “pulsos” con otros nodos mediante Server-Sent Events (SSE).

El mismo binario sirve para un portátil en casa y para un relay en la nube. La diferencia es la variable de entorno `RENDER`.

```
                    ┌─────────────────────────┐
                    │        NodoAlset        │
                    │     (internal/node)     │
                    └───────────┬─────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
  libp2p / DHT            Motor Lisp              Capa neural
  Gossip / streams      (internal/lisp)         (internal/neural)
        │                       │                       │
        └───────────────────────┼───────────────────────┘
                                │
                    ┌───────────▼───────────┐
                    │  Pulsos SSE + HTTP    │
                    └───────────┬───────────┘
                                │
                    ┌───────────▼───────────┐
                    │     Persistencia      │
                    │  Local  o  Supabase   │
                    └───────────────────────┘
```

## Paquetes y responsabilidades

### Punto de entrada — `cmd/prisma-tec`

Llama a `node.Run(port)`. No debería crecer mucho.

### Núcleo — `internal/node`

Orquesta el resto. Aquí se decide el modo local/relay, se monta libp2p, se arranca HTTP y se inicializa la persistencia.

La interfaz hacia Lisp está en `host_adapter.go`: el nodo implementa `nodeiface.Host` para que el intérprete no dependa de campos privados de `NodoAlset`.

### Contrato — `internal/nodeiface`

Lista de operaciones que un “anfitrión” debe ofrecer al motor Lisp (y a futuros módulos): firmar, persistir, crear agentes, publicar en el tópico, etc.

Si añades una capacidad nueva que Lisp deba invocar, el camino habitual es:

1. Ampliar la interfaz `Host`.
2. Implementarla en `host_adapter.go`.
3. Usarla desde `internal/lisp`.

### Lisp — `internal/lisp`

Intérprete completo (parser, evaluador, builtins). Solo ve al nodo a través de `Host`. Los tipos de dominio (agentes, neural) los toma de `internal/agents` y `internal/neural`.

### Agentes — `internal/agents`

Estructuras de agente, módulo, entidad, token y roles. El registro compartido `agents.Global` concentra mapas que antes estaban sueltos en el paquete `node`.

### Neural — `internal/neural`

Estado de neurona, sinapsis, inferencia y memoria. Son tipos de datos; la lógica de propagación sigue viviendo en el nodo y en Lisp.

### PoH — `internal/poh`

Eventos y pruebas de “Proof of Humanity”. El almacén global es `poh.Global`, con API explícita (`Append`, `SetSessionID`, `ClearEvents`) para no tocar campos a ciegas.

### Persistencia — `internal/persistence`

| Pieza | Rol |
|-------|-----|
| `Store` | Interfaz común (guardar/cargar) |
| `LocalStore` | Archivos bajo `alset_data/` |
| `SupabaseStore` | Tablas `alset_agents`, `alset_blocks`, `alset_neural_state`, `alset_kv` |
| `NewFromEnv` | Elige backend según variables de entorno |

Flujo de datos:

- **Carga** al arrancar (`CargarEstado`).
- **Escritura** en operaciones que cambian estado y al apagar (`PersistirLocamente`).

### Pulsos — `internal/pulse` + `node/pulse.go`

Los tipos están en el paquete `pulse`. La lógica de cliente/servidor SSE permanece en el nodo porque está muy ligada al ciclo de vida y a los tópicos de red.

En modo relay no se arrancan clientes de pulsos (evita bucles contra uno mismo). En modo local el nodo se suscribe a los relays conocidos.

## Local frente a relay

| Aspecto | Local | Relay (`RENDER` definido) |
|---------|-------|---------------------------|
| Servidor HTTP | Sí | Sí |
| Servidor de pulsos SSE | Sí | Sí |
| Cliente de pulsos | Sí | No |
| Persistencia | Local o Supabase | Normalmente Supabase |

## Dependencias externas relevantes

- **libp2p** — red P2P, DHT, GossipSub
- **go-cid / multihash** — identificadores de contenido estilo IPFS
- **Supabase (PostgREST)** — persistencia opcional en la nube

No hace falta un daemon de IPFS aparte: el nodo genera CIDs y mantiene un blockstore en memoria (y en persistencia).

## Cómo extender el sistema sin romperlo

1. **Nuevo endpoint HTTP** → `internal/node/http.go` (o, cuando exista, `internal/httpapi`).
2. **Nueva función Lisp** → builtins en `internal/lisp`, usando solo métodos de `Host`.
3. **Nuevo tipo de dominio** → paquete adecuado (`agents`, `neural`, …), no en `main`.
4. **Cambio de persistencia** → implementar `persistence.Store` y registrarlo en la factory.
5. **Nunca** subir tokens ni service keys al repositorio.

## Estado de la modularización

Ya separado y usable:

- Entrypoint, nodo, Lisp, persistencia, agentes, neural, PoH, contrato Host

El paquete `node` ya no es un solo archivo: `init`, `p2p`, `persist`, `neural_ops`, `sync`, `admin`, `modules`, `http`, `pulse`.

Ya extraído a paquetes:

- `httpapi` — registro de rutas + handlers de dominio vía `Backend` (agentes, bloques, DNS, Lisp, IA básica)
- Rutas restantes (pulse SSE, admin, módulos, prism) siguen como HandlerFuncs del nodo montadas por `Mount`
- `pulse` — transporte SSE
- `sync` — tipos de sincronización

Pendiente:

- Más rutas HTTP en `httpapi` (módulos, PoH, sync triggers)
- Setup libp2p puro en `p2p`
- Manager de sync completo fuera de `node`

Se hace por pasos para no mezclar refactors grandes con cambios de comportamiento.


## Tests unitarios

Los tests viven junto al código (`*_test.go`). Priorizan paquetes sin red:

- `internal/persistence` — disco local
- `internal/poh`, `internal/agents` — estado de dominio
- `internal/lisp` — parser y eval con un Host de prueba
- `internal/node` — helpers puros

Comando habitual: `go test ./...`

Integración:

- `internal/node` — mux HTTP real vía `buildHTTPHandler` + `httptest` (crear/listar agentes, peers, recarga de estado)
- `internal/persistence` — ciclo completo en disco; Supabase opcional si hay `SUPABASE_URL` y `SUPABASE_SERVICE_KEY`

## Lisp power layer

`internal/lisp/power.go` registra builtins recuperados de salvas históricas del nodo:

- red y economía: `host-id`, `net-peers`, `set-agent-balance`, `get-agent-root`
- ternario: `embedding`, `ternarizar`, `desternarizar`, `similitud`
- modelos: `crear-capa-lineal`, `crear-modelo`, `inferir`, `entrenar-hebbiano`

Documentación de uso: [LISPAI.md](LISPAI.md)
