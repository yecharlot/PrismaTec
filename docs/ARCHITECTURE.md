# Arquitectura Alset (P.TEC-AN v4.0)

## Layout del código

```
cmd/prisma-tec/main.go          Entrypoint
internal/
  node/
    node.go       Núcleo NodoAlset, Init, P2P, sync, persistencia, Run
    types.go      Agente, NeuralState, Módulos, Sync, etc.
    lisp.go       Motor Lisp completo
    pulse.go      Clientes/servidor de pulsos SSE
    http.go       startHTTPServer y handlers HTTP
  persistence/    Store + Local + Supabase (tablas estructuradas)
docs/
static/
```

## Persistencia Supabase (proyecto Alset)

| Tabla | Contenido |
|-------|-----------|
| alset_agents | Un row por agente |
| alset_blocks | Un row por CID |
| alset_neural_state | Estado neural |
| alset_kv | Nombres DNS + backups |

Env: `SUPABASE_URL`, `SUPABASE_SERVICE_KEY`

## Modos

- **Local** (`RENDER` vacío): cliente de pulsos + servidor
- **Relay** (`RENDER` definido): solo servidor de pulsos

## Roadmap de extracción a paquetes independientes

1. ✅ Monolito → varios archivos en `internal/node`
2. ✅ Persistencia pluggable + tablas estructuradas
3. ⏳ `internal/lisp` como paquete propio (interfaz NodeBackend)
4. ⏳ `internal/pulse`, `internal/httpapi`, `internal/neural`, `internal/p2p`

## Capa nodeiface (extracción futura de Lisp/HTTP)

`internal/nodeiface.Host` define la superficie que el motor Lisp y otros paquetes
necesitan del nodo. `*NodoAlset` implementa esos métodos en `host_adapter.go`.

Lisp permanece en `internal/node/lisp.go` porque usa globals de módulos/entidades/PoH
compartidos. La extracción total requiere mover también esos globals a paquetes propios.

## Mapa de archivos del nodo

| Archivo | Rol |
|---------|-----|
| node.go | Núcleo P2P, sync, persistencia, Run |
| types.go | Tipos + aliases a agents/neural |
| lisp.go | Motor Lisp |
| pulse.go | Pulsos SSE |
| http.go | HTTP API |
| host_adapter.go | Implementación de nodeiface.Host |
