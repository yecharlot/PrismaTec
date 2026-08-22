# MANIFIESTO ALSET-GEN: LA SEMILLA

**Versión:** 1.1  
**Fecha:** 2026-08-22  
**Autor:** Prism@.TEC Core Technology (PTCT)  
**Estado en repo:** G0–G2 sembrados — ver `docs/ALSET_GEN_ROADMAP.md`

---

## Visión (la semilla)

El **gen Alset** es una **célula madre digital**: una semilla fractal que no es ni un programa ni un dato suelto, sino un **patrón de crecimiento**.

- Viaja por la red como una sonda autónoma: no necesita ser alojada como un servicio monolítico.
- Puede alimentarse de **errores y tráfico** (observación no invasiva → hallazgos CID) para adaptar su forma.
- **No ocupa el centro**: no invade; observa, registra en CIDs y comunica sin perturbar.
- Contiene el todo en la parte: **fractalidad** alineada a Zyrion (0 seguir · 1 matizar · 2 sumidero).
- Puede **mutar su RootCID** manteniendo la **identidad ANS** (key estable) y cambiando la **esencia**.
- Escucha **pulsos**; resonancia local, no dominación de la red.
- La inteligencia del ecosistema no está solo en un servidor: está en el **viaje**, la **metamorfosis** y la **resonancia**.

> Alset Mind y Alset-Gen **no son lo mismo**.  
> **Mind** orquesta, dialoga y veta.  
> **Gen** es la célula que viaja, muta y observa.

---

## Principios

| Principio | Descripción |
|-----------|-------------|
| Identidad estable, naturaleza mutable | Key ANS eterna; RootCID efímero. |
| Metamorfosis gobernada | Solo auth válida (G2+) muta RootCID. |
| Memoria inmutable | Hallazgos y transformaciones en CIDs. |
| Viaje autónomo | Ubicación (G1 stub → G3 hop P2P). |
| Resonancia local | Pulsos del nodo sin tumbar la red. |
| Servicio en el viaje | CONSULTA mientras se mueve. |
| No invasivo | Observa y registra; no reescribe estado ajeno. |
| Fractalidad | Patrón mínimo; expansión bajo demanda. |

---

## Núcleo

**Inmutable:** key ANS, origen, historial de RootCIDs.  
**Mutable:** RootCID actual, manifiesto, estado, hallazgos (`findings` / `episode_cids`).

**Órganos locales (0/1/2):** dialog, act, mem, self, ethics, curiosity, humor.

**Pulsos:** CONSULTA, MUTATE_ROOTCID, ESTADO, HALLAZGO, GEN_CREATED, GEN_MUTATED, GEN_TRAVEL.

---

## Ciclo de vida

1. Creación — key + RootCID (manifiesto sellado).  
2. Viaje — ubicación ANS + pulso GEN_TRAVEL + hop HTTP opcional (`target_url` → `/api/gen/arrive`).  
3. Mutación — nuevo RootCID + historial (G2 secret).  
4. Observación — hallazgo CID (errores/tráfico/manual).  
5. Servicio — CONSULTA + órganos.  
6. Resonancia — `ResonateGensOnPulse` selectivo.

---

## Implementación

| Pieza | Ruta |
|-------|------|
| Tipos | `internal/agents/gen.go` |
| Ciclo de vida | `internal/node/gen_lifecycle.go` |
| Semilla (observe, auth, resonancia) | `internal/node/gen_seed.go` |
| Persistencia | `gen_registry.json` + Store `KeyGens` + snapshot CID |

### API

| Método | Ruta |
|--------|------|
| GET | `/api/gen` |
| POST | `/api/gen/create` |
| POST | `/api/gen/mutate` — `auth_note` = `GEN_MUTATE_SECRET` o `BOOTSTRAP_SECRET` si existen |
| POST | `/api/gen/travel` |
| POST | `/api/gen/consult` |
| POST | `/api/gen/observe` |

### Entorno

- `GEN_MUTATE_SECRET` (preferido) o `BOOTSTRAP_SECRET`.  
- Sin secret: modo `open_dev` (solo desarrollo).

---

## Honestidad de la semilla

No es un crawler web mágico ni un LLM. Travel P2P real (G3), Lisp en la forma (G4) y orquestación Mind (G5) siguen en el roadmap. Cada fase debe demostrarse con **curl y CID**.

*Documento vivo — visión única de la semilla Alset en la red.*


## Persistencia autónoma (G3.7)

La semilla no debe depender de un solo contenedor Render.

1. **Frontier package** — JSON content-addressed (`type: alset_gen_frontier_package`) con identidad, historial, HTML de servicio embebido.
2. **`package_cid`** — quien guarda este CID puede **revivir** la célula en cualquier nodo Alset (`POST /api/gen/revive`).
3. **Store `KeyGens`** — registro del nodo (Supabase/local).
4. **Espejo** `static/gens/{name}/` — ayuda en el proceso local.

No es magia: la autonomía es **content-addressed**. Sin el CID ni una copia del bloque, no hay resurrección. Con el CID, no hace falta el nodo original.
