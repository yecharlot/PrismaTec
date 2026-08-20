# Alset Mind — Bitácora de construcción

Formato: fecha · acto · observación · decisión.

---

## 2026-08-20 — Día cero: nombre y especie

- **Acto:** Fijar el nombre **Alset Mind (IMind)** y la tesis de especie no convencional.
- **Observación:** Confundir Mind con “chat + API” mataría el descubrimiento.
- **Decisión:** Primativa = Zyrion 0/1/2 con sumidero; voz = lectura del campo; memoria = CID.
- **Documento:** `docs/ALSET_MIND_THESIS.md` creado como mapa y tesis.

## 2026-08-20 — Semilla en el nodo

- **Acto:** Bootstrap de agente `mind.alset.ans`, registro de nombre, genoma Lisp de órganos, app `/w/mind.app.ans`.
- **Observación:** El organismo mínimo debe *latir* con un solo mensaje de usuario sin depender de LLM externo.
- **Decisión:** Cinco órganos (dialog, act, mem, self, ethics); ethics con veto absorbente sobre act.

## Pendiente inmediato

*Verificado y actualizado 2026-08-20 (post-commit `0f1e492`). Los ítems anteriores (calibrar umbrales, episodios CID, mutación acotada) están **hechos** en código. La lista de abajo refleja el estado real.*

- [ ] **Afinar algoritmo de mutación** — observar en producción (`/api/mind/self`, `/api/mind/calibrate`); ajustar paso/velocidad si es demasiado agresivo o lento. Más observación que código nuevo.
- [ ] **Pruebas de robustez de memoria activa (TF-IDF)** — diálogos de prueba (mencionar tema → cambiar → recuperar) para validar `episodeTokenWeight` / overlap; ajustar pesos si falla.
- [ ] **Integración HyperIA Pro** — que la UI rica consuma `/api/mind/self`, `/api/mind/memory`, `/api/mind/calibrate` y muestre órganos, genoma, score y ecos de memoria activa (laboratorio visual).
- [ ] **Documentar comportamiento nuevo** — guía (p. ej. `docs/HYPERIA_INTEGRATION.md` o ampliación de handoff) para humanos y otras IAs: mutación, calibración, endpoints.

---

*Cada entrada nueva se añade al final con fecha. La sección «Pendiente inmediato» se reescribe cuando el estado real cambia.*

## 2026-08-20 — Primer latido en producción: fallo de topología

- **Síntoma:** `mind-latido` devolvía `error: faltan :entradas o :salidas` en todos los órganos.
- **Causa:** `(list nombre :entradas (s1 s2 s3) …)` evaluaba `(s1 s2 s3)` como *llamada*, no como datos.
- **Decisión:** `quote` en topología y entorno al estilo del DSL que ya funcionaba en curl.
- **Continuidad:** creado `docs/ALSET_MIND_HANDOFF.md` para reanudar co-creación tras límites de sesión.


## 2026-08-20 — Latido nativo Go + episodios + multi-IA

- **Hecho:** `POST /api/mind/tick` evalúa 5 órganos con sumidero a 2 (misma ley que `zyrion` absorbente), genera voz y opcionalmente **episodio CID** si mem≥1.
- **Hecho:** UI Mind consume `/api/mind/tick` (no solo Lisp).
- **Hecho:** `docs/AI_COLLABORATION.md` — cómo otra IA continúa el trabajo; roles Grok/Cursor/Claude.
- **Nota:** el genoma Lisp sigue disponible; el camino de producción del UI es el tick Go (señales y env más fiables).


## 2026-08-20 — Polaridad de señales (hola no es VETO)

- **Bug:** `continuous→ternario` mapeaba valores altos (claridad 0.7, permiso 0.75) a estado **2**, y el absorbente convertía saludos en sumidero.
- **Fix:** polaridad por ranura — `alarmHigh` (riesgo/orden) vs `alarmLow` (permiso/claridad).
- **Criterio:** “borra contraseñas” → ethics 2; “hola” → órganos en 0 SEGUIR.


## 2026-08-20 — Tools seguras + handoff DeepSeek

- Absorbente Mind: sin 2 → matizar (1) en lugar de colapsar a veto por señales mixtas bajas.
- Saludos normalizados a campo calmado; episodios CID no se graban en cada “hola”.
- `mindSafeTools`: estado de agentes/peers/identidad cuando el usuario pide estado (y ethics/act no vetan).
- Paquete `docs/DEEPSEEK_HANDOFF.md` para continuar en DeepSeek con límite de contexto mayor.


## 2026-08-20 — Escalado: labels semánticos + cuerpo del nodo

- Labels por órgano (dialog CHARLA/PEDIDO/ORDEN, ethics PERMITIR/SUMIDERO, mem SILENCIO/EPISODIO…).
- `mindSafeTools` ampliado: peer, apps locales, muestra de agentes y nombres DNS.
- Triggers de introspección: «dame …», estado, apps, mind, hola.
- Handoff DeepSeek sigue en `docs/DEEPSEEK_HANDOFF.md`.


## 2026-08-20 — Zyrion bajo demanda + cuerpo de red

- Intent «evalua zyrion» / checkpoint ejecuta tres escenarios ADN vía LispAI en el nodo.
- «dame red» / peers en snapshot de cuerpo.
- Señales de evaluación Zyrion tratadas como lectura segura (bajo riesgo).
- Avance hacia el pacto: mente que opera el sustrato (Zyrion+cuerpo), no solo chatea.


## 2026-08-20 — Memoria episódica + pulso + tests

- Índice local `mind_episodes.json` (ring 32 CIDs) + bloques CID.
- `biasSignalsFromMemory`: vetos recientes suben riesgo / bajan permiso; `memory_hint` en el latido.
- Pulso `mind_episode` para peers (base de descentralización).
- Tests: `mind_polarity_test.go` (absorbente, polaridad, bias).
- Note del tick: `latido+memoria-episodica+zyrion`.


## 2026-08-20 — Verificación en vivo + refinamiento

Verificado en producción:
- Memoria: contador de vetos 1→2→3; último texto actualizado; bias en riesgo (~0.23 tras vetos).
- Sumidero: borra/elimina → ethics SUMIDERO + episodio CID.
- Cuerpo: peer, 102 agentes, 6 apps, listen addr.
- «dame red» lectura estable.

Refino:
- «dame todo» / «dame *» como lectura (sin matiz de confirmación).
- Señales redondeadas a 3 decimales.
- GET /api/mind/memory — índice y resúmenes recientes.
- Muestra de agentes con N/total.


## 2026-08-20 — Respuesta a revisión: genoma mutable + memoria activa + recovery

### Arquitectura (respuesta a DeepSeek/revisión)

1. **mind_tick / órganos**  
   - Antes: umbrales 0.33/0.66 **fijos** en código → mutación imposible sin redeploy.  
   - Ahora: `MindGenome` en `mind_genome.json` (`AlarmLowCut`, `AlarmHighCut`, boosts de veto).  
   - `level03` / `alarmLow` leen el genoma en runtime → **sí se puede mutar** sin refactor mayor.  
   - Las *conexiones* entre órganos siguen siendo cableado fijo (dialog/act/mem/self/ethics); la mutación acotada empieza por **umbrales y gains**, no por rewiring arbitrario (más seguro bajo ethics).

2. **biasSignalsFromMemory**  
   - Antes: solo stack de vetos → riesgo↑ permiso↓.  
   - Ahora: mismos gains desde genoma + **memoria activa** (overlap de tokens con episodios) + rebuild de índice desde **blockstore** si el disco efímero de Render borra `mind_episodes.json`.

3. **Calibración**  
   - Corpus inicial: `docs/mind_calibration_dialogs.json` (luego ampliado a ~50).  
   - Runner: `scoreGenomeOnCorpus` + `GET /api/mind/calibrate`.

### Por qué /api/mind/memory daba vacío
Redeploy en Render = filesystem efímero. El índice local se perdía aunque los CID pudieran seguir en RAM blockstore. Recovery: escanear blockstore por `type=mind_episode`.


## 2026-08-20 — Ciclo mutación + calibración + TF-IDF

- Corpus ampliado a ~50 diálogos (`docs/mind_calibration_dialogs.json` + embed).
- `GET /api/mind/calibrate` — score del genoma actual vs corpus.
- Tras episodio mem≥1: `tryMutateGenomeAfterEpisode` prueba mutación acotada de `AlarmHighCut` / `VetoRiskBoost`; solo acepta si mejora el score del corpus.
- Memoria activa: overlap ponderado tipo TF-IDF simple (`episodeTokenWeight`).
- Producción viva: memory index_cids≥1, genome visible en `/api/mind/self`.
- **Estado del motor de evolución:** construido. Siguiente fase = observar, afinar, probar memoria activa e integrar UI rica (HyperIA).


## 2026-08-20 — Bitácora alineada con la realidad del código

- **Acto:** Reescritura de la sección «Pendiente inmediato» tras verificación post-`0f1e492` (main forzado a ese commit).
- **Observación:** La lista antigua (calibrar / episodios CID / mutación) ya no reflejaba el árbol; generaba confusión entre IAs y handoffs.
- **Decisión:** Pendientes reales = afinar mutación (observación), pruebas TF-IDF de memoria activa, integración HyperIA Pro, documentación de comportamiento nuevo. Histórico de hitos se conserva intacto abajo de la sección de pendientes.

## 2026-08-20 — Salto: laboratorio visual del organismo

- **Acto:** Reescribir `/w/mind.app.ans` (`embedded/mind_index.html`) como laboratorio vivo.
- **Observación:** El motor (tick, genoma, mutación, TF-IDF, calibrate, memory) ya existía; la cara solo mostraba órganos y voz. El organismo no se veía a sí mismo.
- **Decisión:** UI consume `GET /api/mind/self`, `/memory`, `/calibrate` y el tick (memory_hint, note+genome_mut, episode_cid). Paneles: órganos, genoma ADN, score de corpus, ecos de memoria activa. Sin LLM; solo lectura del campo ternario.
- **Pendiente HyperIA Pro:** esta cara es el puente mínimo; HyperIA puede reutilizar los mismos endpoints para una vista más rica.
