# Alset-Gen — Roadmap

Alineado a `docs/ALSET_GEN_MANIFESTO.md`. **No sustituye a Alset Mind.**

| Fase | Contenido | Estado |
|------|-----------|--------|
| **G0** | Manifiesto + tipos `AlsetGen` | Hecho |
| **G1** | Crear / mutar / viajar (stub) / consultar + API + persistencia + pulsos | Hecho en prod |
| **G1.5** | Observación no invasiva (`/api/gen/observe` + resonancia a pulsos) | Hecho 2026-08-22 |
| **G2** | Auth de mutación (`GEN_MUTATE_SECRET` / `BOOTSTRAP_SECRET`) | Hecho (sin secret = open_dev) |
| **G2.5** | Persistencia durable `KeyGens` + snapshot CID (sobrevive redeploy) | Hecho 2026-08-22 |
| **G3** | Travel + ANS location + HTTP hop `/api/gen/arrive` | Hecho 2026-08-22 |
| **G3.5** | Explore frontera URL (sin nodo Alset) + Mind gestiona gens | Hecho 2026-08-22 |
| **G3.6** | Servicio en frontera local `/work/{gen}/` y `/g/{gen}/` | Hecho 2026-08-23 |
| **G3.7** | Persistencia autónoma: frontier package CID + revive | Hecho 2026-08-23 |
| **G3.8** | Daemon `cmd/alset-gen` (HTTP + libp2p opcional + resolve) | Hecho 2026-08-23 |
| **G3.9** | Explore+diálogo en daemon; announce → Mind localiza | Hecho 2026-08-23 |
| **G3.10** | Pulse-over-UDP + BEACON/CARGO en borde de red | Hecho 2026-08-23 |
| **G3.11** | CARGO store-and-forward entre bordes + análisis torrente | Hecho 2026-08-23 |
| **G3.12** | Publish by-cid + DNS TXT resolve + package-url | Hecho 2026-08-23 |
| **G3.13** | Cloudflare Worker + Durable Object (edge torrente) | Hecho 2026-08-23 |
| **G4** | Lógica Lisp / forma ligada a RootCID + fractal | Pendiente |
| **G5** | Orquestación desde Alset Mind (tool bajo ethics) | Pendiente |

## Principio

La semilla **observa y registra**; no es un crawler agresivo ni un LLM. Cada paso debe poder demostrarse con curl y CID.

## API

| Método | Ruta | Body |
|--------|------|------|
| GET | `/api/gen` | — |
| POST | `/api/gen/create` | `{ "key", "root_cid?", "type?", "description?" }` |
| POST | `/api/gen/mutate` | `{ "key", "new_root_cid", "auth_note?" }` |
| POST | `/api/gen/travel` | `{ "key", "target?" }` |
| POST | `/api/gen/consult` | `{ "key", "stimulus?" }` |
| POST | `/api/gen/observe` | `{ "key", "source?", "detail?" }` |

## Producción

- Host: `https://prismatec.onrender.com`
- Definir `GEN_MUTATE_SECRET` en Render para cerrar mutaciones abiertas.
