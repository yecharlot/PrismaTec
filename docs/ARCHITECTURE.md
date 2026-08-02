# Arquitectura de PrismaTec / Alset (P.TEC-AN v4.0)

## Visión general

Alset es una red peer-to-peer que combina:

- Comunicación P2P (libp2p)
- Motor de evaluación Lisp embebido
- Capa de IA distribuida (modelo de spikes + sinapsis Hebbianas)
- Sistema de pulsos SSE para resiliencia
- Persistencia pluggable (Local / Supabase)
- Servidor HTTP + panel de administración

Existen dos modos de ejecución del mismo binario:

| Modo          | Detección              | Comportamiento principal                     |
|---------------|------------------------|----------------------------------------------|
| **Local**     | `RENDER` no está definida | Cliente de pulsos + servidor completo       |
| **Relay**     | `RENDER` está definida    | Solo servidor de pulsos + sync completa     |

## Diagrama de alto nivel

```
                    ┌─────────────────────────────────────┐
                    │           NodoAlset                 │
                    │  (internal/node)                    │
                    └──────────────┬──────────────────────┘
                                   │
          ┌────────────────────────┼────────────────────────┐
          │                        │                        │
          ▼                        ▼                        ▼
   ┌─────────────┐         ┌─────────────┐         ┌─────────────────┐
   │   libp2p    │         │    Lisp     │         │  Neural Layer   │
   │  Host+DHT   │         │  Evaluator  │         │ Spikes/Sinapsis │
   │  GossipSub  │         │             │         │ Memoria / Inf.  │
   └─────────────┘         └─────────────┘         └─────────────────┘
          │                        │                        │
          └────────────────────────┼────────────────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │     Sistema de Pulsos SSE   │
                    │   /api/pulse  (servidor)    │
                    │   pulseClients (solo Local) │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │   Capa de Persistencia      │
                    │  (internal/persistence)     │
                    │  Local  ←→  Supabase        │
                    └─────────────────────────────┘
```

## Paquetes (estado actual y objetivo)

### Ya implementados / preparados

- `internal/persistence`
  - `Store` interface
  - `LocalStore` (disco)
  - `SupabaseStore` (listo para credenciales)
  - Factory basada en variables de entorno

- `cmd/prisma-tec` → punto de entrada limpio

- `internal/node` → contiene todavía la mayor parte de la lógica (migración incremental)

### Próximas extracciones (orden recomendado)

1. `internal/lisp` – motor completo (parser + evaluator)
2. `internal/pulse` – SSE subscribers + clients
3. `internal/neural` – tipos y lógica de spikes / inferencia / memoria
4. `internal/p2p` – setup de host, DHT, tópicos, stream handlers
5. `internal/agents` – Agente, Modulo, Entidad, roles, tokens
6. `internal/blocks` – CID generation, blockstore, IPFS-like helpers
7. `internal/sync` – SyncManager, quick/full sync
8. `internal/httpapi` – todos los handlers HTTP + admin panel

## Persistencia

Se eliminó completamente `GitHubPersistence`.

El nodo ahora depende de la interfaz:

```go
type Store interface {
    Save(ctx context.Context, key string, data []byte) error
    Load(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
    Close() error
}
```

Claves estándar:

- `alset_state.json`
- `alset_names.json`
- `blocks.json`
- `neural_state.json`

## Decisiones de diseño importantes

- **Un solo binario** para Local y Relay (diferenciado solo por env).
- **Sandbox mental**: cualquier evolución futura del código debe hacerse en rama + PR.
- **Persistencia pluggable** para poder cambiar de backend sin tocar la lógica de negocio.
- **Monolito controlado**: se acepta que `internal/node` sea grande temporalmente mientras se extraen paquetes de forma segura.

## Seguridad

- Tokens y service keys **nunca** van en el repositorio.
- Preferir variables de entorno o secretos de la plataforma (Render, etc.).
- El antiguo archivo `github_config.json` debe considerarse comprometido y no usarse.

## Cómo extender

1. Añadir nueva funcionalidad → preferir un paquete nuevo bajo `internal/`.
2. Cambios de persistencia → implementar la interfaz `Store`.
3. Nuevos endpoints HTTP → colocarlos en el futuro `internal/httpapi`.
