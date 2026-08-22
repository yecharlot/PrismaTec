# Alset-Gen — Roadmap de implementación

Alineado a `docs/ALSET_GEN_MANIFESTO.md`. **No sustituye a Alset Mind.**

| Fase | Contenido | Estado |
|------|-----------|--------|
| **G0** | Manifiesto en repo + tipos `AlsetGen` | Hecho 2026-08-22 |
| **G1** | Crear / mutar / viajar (stub) / consultar + API + persistencia local + pulsos | Hecho 2026-08-22 |
| **G2** | Auth firme de mutación (firma operador / Mind tool) | Pendiente |
| **G3** | Travel real entre peers + ANS de ubicación | Pendiente |
| **G4** | Lógica Lisp ligada a RootCID + fractal implosivo | Pendiente |
| **G5** | Orquestación desde Alset Mind (tool segura bajo ethics) | Pendiente |

## API G1

| Método | Ruta | Body |
|--------|------|------|
| GET | `/api/gen` | — |
| POST | `/api/gen/create` | `{ "key", "root_cid?", "type?", "description?" }` |
| POST | `/api/gen/mutate` | `{ "key", "new_root_cid", "auth_note?" }` |
| POST | `/api/gen/travel` | `{ "key", "target?" }` |
| POST | `/api/gen/consult` | `{ "key", "stimulus?" }` |

## Principio

Mind sigue intacto. Gen es registro paralelo (`n.gens`) con espejo opcional en `agentes` para balance/root.
