> **Archivo histórico.** La documentación canónica es:
> - [README.md](../README.md) — mapa del repo
> - [HANDOFF.md](HANDOFF.md) — estado y gaps
> - [GUIA.md](GUIA.md) — ayuda operativa
>
> Si este texto contradice el HANDOFF, **gana el HANDOFF**.

---
# Trabajar con Alset Mind y Alset Gen juntos

**Documento canónico de operación conjunta.**  
Actualizado: 2026-08-24 · Repo `yecharlot/PrismaTec`

Si solo lees un doc para usar ambos sistemas, **es este**.

---

## 1. Quién es quién

| | **Alset Mind** | **Alset Gen** |
|--|----------------|---------------|
| Rol | Cerebro del nodo: percibe, juzga 0/1/2, habla, recuerda | Célula: identidad ANS, RootCID, puede viajar, explorar, guardar CIDs |
| Motor | Órganos ternarios + genoma (no LLM) | Estado local + edge opcional |
| Memoria | Episodios CID del diálogo / hechos | `EpisodeCIDs` si misión **memory** |
| Ética | Órgano **ethics** (puede vetar) | Organs locales; no bypasea ethics de Mind |

**Regla:** Mind **orquesta**. Gen **ejecuta / reside / almacena**. El usuario habla con Mind; Mind habla con el gen.

```text
Tú  →  Mind (tick)  →  Gen (local o Cloudflare)
         │                │
         └─ episode CID   └─ dialogue / explore / pins
```

---

## 2. Qué pueden hacer **juntos**

| Capacidad | Cómo |
|-----------|------|
| **Crear una célula** | Mind: «crea gen sonda» → registro en el nodo |
| **Dialogar con un gen** | «pregunta al gen sonda: quién eres» → Mind retransmite la voz del gen |
| **Despachar al borde** | «despacha gen sonda a cloudflare» → reach en Worker |
| **Memoria distribuida** | «crea gen memoria mem-nodo» + «salva en gen mem-nodo: …» |
| **Vincular recuerdo de Mind** | «vincula memoria» → último episodio → gen-memoria |
| **Explorar URL** | «explora gen sonda https://ejemplo.com» |
| **Listar** | «lista genes» / «lista genes memoria» |
| **Código (plantillas)** | «genera código handler http en go» (Mind; ethics veta lo destructivo) |
| **Estado del puente** | `GET /api/mind/self` → campo `gen_bridge` |

**Qué no hacen juntos:** no son un LLM compartido; no mutan RootCID sin secret; no guardan passwords en gen-memoria.

---

## 3. Dónde corre cada cosa (producción)

| Pieza | URL |
|-------|-----|
| Nodo + Mind + Gen API | https://prismatec-4u5c.onrender.com |
| Lab Mind | https://prismatec-4u5c.onrender.com/w/mind.app.ans |
| Red / edge | https://alset-network.lhmolam-877.workers.dev |
| Store DO | `GET …/api/store/info` → `species: AlsetStoreDO` |

Env típico del nodo: `ALSET_CLOUDFLARE_NETWORK`, `ALSET_CF_STORE_URL`, `ALSET_PERSIST=cloudflare`.  
Opcional: `ALSET_AUTO_PIN_MEM=1` (cada episodio Mind se ancla en `mem-nodo`).

---

## 4. Flujo de trabajo recomendado (mano a mano)

### A. Sesión diaria con Mind
1. Abre el lab o `POST /api/mind/tick`.
2. Charla / hechos («me llamo…») → memoria CID de Mind.
3. Pregunta de red: «qué es la red alset», «cómo aprendes».

### B. Crear y usar un gen
```text
crea gen sonda-alpha
pregunta al gen sonda-alpha: quién eres
despacha gen sonda-alpha a cloudflare
dile al gen sonda-alpha: estado
```

### C. Salva en gen-memoria
```text
crea gen memoria mem-nodo
salva en gen mem-nodo: decisión de arquitectura X
vincula memoria
lista genes memoria
```

### D. API directa (scripts / otras IAs)
```bash
BASE=https://prismatec-4u5c.onrender.com

# Mind
curl -s -X POST $BASE/api/mind/tick -H 'Content-Type: application/json' \
  -d '{"text":"lista genes"}' | jq -r .voice

# Gen memoria
curl -s -X POST $BASE/api/gen/memory/create -d '{"key":"mem-nodo"}'
curl -s -X POST $BASE/api/gen/memory/save \
  -d '{"key":"mem-nodo","text":"nota operativa","note":"ops"}'

# Diálogo gen
curl -s -X POST $BASE/api/gen/dialogue \
  -d '{"key":"sonda-alpha","text":"quién eres"}'

# Edge
curl -s https://alset-network.lhmolam-877.workers.dev/api/store/info
curl -s https://alset-network.lhmolam-877.workers.dev/api/network/gens
```

---

## 5. Cuándo usar Mind solo / Gen solo / ambos

| Situación | Usar |
|-----------|------|
| Charla, identidad, ethics, corpus, codegen plantilla | **Mind** |
| Célula con misión, explore, edge, paquete CID | **Gen** |
| Recordar + querer que viaje/se ancle en célula | **Mind + gen-memoria** |
| Persistencia de bloques del nodo | **Store DO (CF)** vía nodo |

---

## 5b. Director de diálogo (2026-08-24)

Prioridad fija en cada latido: **ethics → gen/tools → matemática (LispAI) → codegen → memoria → corpus → charla**.  
Curiosity/humor **no** se pegan si hubo tool/código/cálculo.  
Hilo: último gen / explore / código para «qué significa esto».  
Normalización de typos frecuentes del usuario.

## 6. Dónde estamos / a dónde / qué falta

| | |
|--|--|
| **Ahora** | Puente voz+API; gen-memoria; edge+Store DO desplegados; Mind en Render |
| **Queremos** | Memoria de red coherente; auto-pin habitual; lab con panel gen |
| **Falta** | Panel UI de genes en lab; auto-dispatch gen-memoria; codegen por fragmentos más rico |

---

## 7. Índice de documentación (sin ruido)

### Canónicos (léelos en este orden)
1. **Este archivo** — operación conjunta  
2. `ALSET_STATUS_HANDOFF.md` — estado prod, gaps, reglas  
3. `ALSET_MIND_THESIS.md` — especie Mind  
4. `ALSET_GEN_MANIFESTO.md` — especie Gen  
5. `ALSET_CF_STORE.md` — persistencia DO  
6. `ALSET_MIND_CONSTRUCTION_LOG.md` — bitácora cronológica  

### Especializados (solo si profundizas)
| Doc | Tema |
|-----|------|
| `ALSET_MIND_GEN_BRIDGE.md` | Detalle técnico del puente (resumido aquí) |
| `ALSET_GEN_MEMORY.md` | Solo gen-memoria |
| `ALSET_NETWORK_LIVE.md` | URLs y pruebas edge |
| `ALSET_MIND_ROADMAP.md` / `TRAINING_PLAN.md` | Fases Mind |
| `ALSET_GEN_ROADMAP.md` | Fases Gen |
| `MIND_LAB.md` | UI lab |
| `ALSET_GEN_DAEMON.md` / `PUBLISH.md` | Daemon y publish CID |

### Redundantes / secundarios (no borrar aún; no empieces por ellos)
- `ALSET_MIND_HANDOFF.md` → preferir **STATUS_HANDOFF**  
- `ALSET_GEN_CLOUDFLARE.md` + `ALSET_NETWORK_CLOUDFLARE.md` → preferir **NETWORK_LIVE** + **CF_STORE**  
- `ALSET_GEN_NETWORK_PATH.md` / `TORRENT.md` → análisis de camino de red; no operativos día a día  

---

*Cualquier IA o programador: empieza por la §4. Si algo falla, mira `gen_bridge` en `/api/mind/self` y `/api/store/info`.*

## 8. Sondas: eliminar, retornar, explorar y aprender (2026-08-24)

| Acción | Frase / API |
|--------|-------------|
| Eliminar | «elimina gen genesis» · `POST /api/gen/delete` |
| Retornar (explore o CF) | «retorna gen genesis» · `POST /api/gen/return` |
| Explorar | «envía al gen X a explorar https://…» |
| Sonda automática | Si no hay corpus, Mind puede crear `scout-*`, explorar Wikipedia ES, integrar hallazgo, anclar en memoria y **borrar** la sonda (`ALSET_SCOUT_EPHEMERAL`, default on) |

El gen es la **herramienta de frontera** de Mind: no sustituye ethics ni el corpus curado; aporta hallazgos cuando el organismo no sabe.
