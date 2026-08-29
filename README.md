# PrismaTec / Alset

Red peer-to-peer en **Go**: nodo libp2p, agentes, apps por CID, **LispAI**, lógica ternaria **Zyrion**, **Alset Mind** (organismo de decisión) y **Alset Gen** (células / sondas).

> **No es un LLM.** Mind evalúa órganos 0/1/2, recuerda en CID y orquesta genes. La voz sale de corpus, memoria, plantillas y herramientas — no de predicción de tokens.

---

## Documentación (solo 3 entradas)

| Doc | Rol |
|-----|-----|
| **[README.md](README.md)** (este archivo) | Mapa del repo, módulos, arranque |
| **[docs/HANDOFF.md](docs/HANDOFF.md)** | Estado actual, tip de `main`, gaps, reglas |
| **[docs/GUIA.md](docs/GUIA.md)** | Cómo usar Mind, Gen, API, sesiones, pruebas |

El resto de `docs/ALSET_*.md` históricos queda como **archivo**; no empezar por ahí.

---

## Mapa modular del repo

```text
cmd/prisma-tec/          → binario del nodo
internal/node/           → corazón: HTTP, Mind, Gen, red, persistencia
  mind_*.go              → Alset Mind (tick, sesiones, diálogo, razón, scout…)
  gen_*.go               → Alset Gen (lifecycle, explore, dispatch, memoria)
  embedded/              → UI lab + mind_knowledge.json (corpus)
internal/lisp/           → LispAI
internal/persistence/    → Store: local | Supabase | Cloudflare DO
cloudflare/alset-gen-worker/ → red edge + AlsetStoreDO
docs/                    → HANDOFF + GUIA (+ archivo histórico)
scripts/                 → baterías de diálogo / utilidades
```

| Módulo | Qué es | Entrada típica |
|--------|--------|----------------|
| **Nodo** | API HTTP + libp2p | `go run ./cmd/prisma-tec` → `:8080` |
| **Mind** | Órganos + genoma + voz | `POST /api/mind/tick` |
| **Gen** | Células ANS + sondas | `POST /api/gen/*` |
| **LispAI** | Evaluación Lisp / ternaria | integrado en nodo |
| **Store** | Persistencia bloques/KV | env `ALSET_PERSIST` / CF / local |
| **Edge** | Worker Cloudflare | `alset-network.*.workers.dev` |

---

## Arranque local

```bash
git clone https://github.com/yecharlot/PrismaTec.git
cd PrismaTec
go run ./cmd/prisma-tec
```

- API: `http://localhost:8080`
- Lab Mind: `http://localhost:8080/w/mind.app.ans`
- Salud: `GET /api/v2/info`

### Mind con sesión (memoria aislada por cliente)

```bash
curl -s -X POST http://localhost:8080/api/mind/tick \
  -H "Content-Type: application/json" \
  -d '{"text":"me llamo Diego","session":"s-a"}' | jq -r .voice

curl -s -X POST http://localhost:8080/api/mind/tick \
  -H "Content-Type: application/json" \
  -d '{"text":"cómo me llamo?","session":"s-a"}' | jq -r .voice
```

Detalle de frases, genes y sondas: **[docs/GUIA.md](docs/GUIA.md)**.

---

## Alset Mind (resumen)

- **7 órganos:** dialog, act, mem, self, ethics, curiosity, humor  
- **Memoria:** episodios CID + **sesiones** (`session` / header `X-Mind-Session`)  
- **Diálogo:** director de prioridad, flujo de temas, plantillas, batería de tests  
- **Razón:** silogismos ternarios (`mind_reason`)  
- **Scout:** si no hay corpus, sonda gen → web (Wikipedia) → ancla hallazgo  
- **Codegen:** plantillas bajo ethics (no generación libre tipo LLM)

## Alset Gen (resumen)

- Célula con key ANS + RootCID  
- Explorar URL, despachar a Cloudflare, **retornar**, **eliminar**  
- Gen-memoria: anclas CID  
- Mind orquesta; ethics de Mind manda  

---

## Persistencia y borde

| Backend | Activación |
|---------|------------|
| Cloudflare DO | `ALSET_PERSIST=cloudflare` + `ALSET_CF_STORE_URL` |
| Supabase | `SUPABASE_URL` + key (fallback) |
| Local disco | sin env cloud |

Worker: `cloudflare/alset-gen-worker` (`npx wrangler deploy`).

---

## Principios

1. Ternario y CID, **no** wrapper LLM  
2. Ethics puede vetar  
3. Deploy Render **manual** (auto-deploy OFF en cuentas free)  
4. Un handoff, una guía, este README  

Estado y gaps: **[docs/HANDOFF.md](docs/HANDOFF.md)**.
