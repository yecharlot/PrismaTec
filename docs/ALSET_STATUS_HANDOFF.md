# Alset / PrismaTec — Estado, destino y huecos (handoff)

**Actualizado:** 2026-08-23  
**Repo:** `https://github.com/yecharlot/PrismaTec`  
**Branch:** `main`  
**Tip al deploy de esta fecha:** `2a55ecb` (gen-memoria) + commits previos de store CF, corpus red, codegen, curiosity.

Este documento es la **fuente de orientación** para cualquier IA o programador. La bitácora detallada sigue en `ALSET_MIND_CONSTRUCTION_LOG.md`.

---

## 1. Dónde estamos

### Especie
- **Alset Mind:** organismo ternario Zyrion (0/1/2), no LLM.
- **Alset Gen:** células con key ANS + RootCID; viaje, explore, edge.
- **Nodo PrismaTec:** proceso Go (HTTP + libp2p + LispAI + Mind + Gen).

### Producción (cuenta Render **Elohim** / `lhmolam@gmail.com`)

| Servicio | URL | Auto-deploy |
|----------|-----|-------------|
| **PrismaTec** | https://prismatec-4u5c.onrender.com | **OFF** (solo manual) |
| **PrismatecAlsetMind** | https://prismatecalsetmind-vrzg.onrender.com | **OFF** |
| Edge red | https://alset-network.lhmolam-877.workers.dev | Cloudflare Worker |

Lab Mind: `https://prismatec-4u5c.onrender.com/w/mind.app.ans`

### Implementado en código (resumen)

| Área | Estado |
|------|--------|
| 7 órganos + genoma 1.1 + feedback | Sí |
| Memoria CID hablada + lab UI | Sí |
| Corpus ~178 (red Alset, aprendizaje, polímata) | Sí |
| Curiosity afinada | Sí |
| Codegen plantillas + ethics + CID (`mind_codegen`) | Sí (no es LLM) |
| Gen orquestación por voz | Sí |
| **Gen-memoria** (misión memory, pin CID, API) | Sí |
| **CloudflareStore** + `AlsetStoreDO` en Worker (código) | Sí (P0/P1) |
| Factory: CF → Supabase → local | Sí |
| Env prod: `ALSET_PERSIST=cloudflare`, `ALSET_CF_STORE_URL` | Configurado en Render |

### Política de deploy
- Auto-deploy **desactivado** para no quemar horas free.
- Deploy **solo** bajo confirmación explícita del operador.

---

## 2. A dónde queremos llegar

1. **Persistencia estable** en Cloudflare DO (sin depender de Supabase inestable).
2. **Genes memoria** como salva distribuida de CIDs de la red.
3. **Mind experto** de su propia red + aprendizaje ternario continuo.
4. **Codegen** más general por **composición de fragmentos** (sigue sin LLM).
5. Opcional: IPFS global; G4 Lisp en RootCID; enjambre de peers.

Visión larga: organismo digital ético, transparente, con memoria por contenido y borde global — **no** un chatbot estadístico.

---

## 3. Qué falta para lograrlo

| Hueco | Acción |
|-------|--------|
| **Worker con `AlsetStoreDO` en CF** | `cd cloudflare/alset-gen-worker && npx wrangler deploy` (migración v2-store). Sin esto, `/api/store/*` puede fallar y el nodo hará fallback a Supabase/local. |
| **Probar store** | `curl …/api/store/info` → `species: AlsetStoreDO` |
| **Probar Mind en prod** | tick, gen memoria, codegen, lab |
| **Codegen general** | Más fragmentos / ensamblador (no hecho) |
| **Gen-memoria en edge** | Anunciar gens memoria al Worker (parcial; local+registry sí) |
| **Slug corto** `prismatec.onrender.com` | No reclamable aún; usar `prismatec-4u5c` o dominio propio |
| **Supabase** | Dejar de ser crítico tras validar CF store |

---

## 4. Cómo probar en producción

```bash
# Salud
curl -s https://prismatec-4u5c.onrender.com/api/v2/info

# Mind
curl -s https://prismatec-4u5c.onrender.com/api/mind/self | jq .
curl -s -X POST https://prismatec-4u5c.onrender.com/api/mind/tick \
  -H 'Content-Type: application/json' \
  -d '{"text":"qué es la red alset"}' | jq -r .voice

# Gen memoria
curl -s -X POST https://prismatec-4u5c.onrender.com/api/gen/memory/create \
  -H 'Content-Type: application/json' -d '{"key":"mem-nodo"}'
curl -s -X POST https://prismatec-4u5c.onrender.com/api/gen/memory/save \
  -H 'Content-Type: application/json' \
  -d '{"key":"mem-nodo","text":"prueba salva produccion","note":"handoff"}'

# Codegen
curl -s -X POST https://prismatec-4u5c.onrender.com/api/mind/tick \
  -d '{"text":"genera código handler http en go"}' | jq -r .voice

# Edge
curl -s https://alset-network.lhmolam-877.workers.dev/api/network/gens
curl -s https://alset-network.lhmolam-877.workers.dev/api/store/info
```

---

## 5. Docs de referencia

| Doc | Contenido |
|-----|-----------|
| `ALSET_MIND_THESIS.md` | Especie |
| `ALSET_MIND_ROADMAP.md` / `TRAINING_PLAN.md` | Fases Mind |
| `ALSET_GEN_MANIFESTO.md` / `ROADMAP.md` | Genes |
| `ALSET_GEN_MEMORY.md` | Gen-memoria |
| `ALSET_CF_STORE.md` | Store DO |
| `ALSET_NETWORK_LIVE.md` | Edge CF |
| `MIND_LAB.md` | Lab UI |
| `ALSET_MIND_CONSTRUCTION_LOG.md` | Bitácora cronológica |

---

## 6. Reglas de oro (no negociables)

1. **No LLM** como motor de decisión o voz.
2. **Ethics** es el único sumidero que absorbe acción.
3. **No romper** Sales/Vero/módulos ajenos al mind/gen.
4. **Documentar** cada hito en bitácora + este handoff.
5. **Deploy manual** y escaso en Render free.

---

*Fin del handoff. Actualizar este archivo en cada hito de producción.*
