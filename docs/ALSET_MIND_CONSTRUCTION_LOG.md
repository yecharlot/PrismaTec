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

## 2026-08-21 — Corpus de programación (bloque B)

- **Acto:** Primer bloque curado prog_lisp / prog_go / prog_ethics / prog_alset en `mind_knowledge.json` (19 entradas).
- **Alcance:** evaluar-zyrion+quote, defun, agentes, mind tick API, paquetes Go del nodo, handlers, context, CID, mutex, ethics de código, RootCID/apps, checkpoint, cómo ampliar corpus, tests.
- **No es:** generador de módulos enteros por predicción de tokens. Es recuperación simbólica determinista + narrativa para ethics.
- **Test:** `TestProgrammingCorpusBlock`.

## 2026-08-21 — Curiosidad + humor activos en la voz + corpus filosófico

- **Acto:** curiosity/humor dejan de ser solo labels: se **añaden** a la voz (pregunta abierta / tinte ligero) sin sustituir conocimiento ni identidad.
- **Corpus:** entradas humano, consciencia, existencia, mente, ética, pensamiento, metáfora, mago/varita.
- **Genoma:** CuriosityCut 0.40, HumorCut 0.30 por defecto.
- **Decisión:** órganos blandos colorean el campo; no generan tokens libres ni anulan ethics.
- **Tests:** TestCuriosityAndHumorActive + regresión de diálogo.

## 2026-08-21 — Diálogo: identidad, hechos y sin sobre-veto

- **Síntoma (conversación real):** «cuántos órganos» genérico; «cuál es tu nombre» devolvía el nombre del usuario; «crea un agente» caía en SUMIDERO por bias de vetos previos; filosofía terminaba en «¿actuar sobre el nodo?»; hechos tipo «la rana es naranja…» no se anclaban bien.
- **Causa:** `speakFromMemory` trataba cualquier «nombre» como nombre del usuario; bias de veto contaminaba charla calmada; `orden` alto en «crea» activaba ethics absorbente; voz de matiz aplicada a diálogo puro.
- **Decisión:** separar `isAskingMindName` vs memoria de usuario; veto-bias solo si el texto actual es destructivo; `isConstructiveOrder` / `isWorldFact` / `isPureDialogue`; voz de órganos y filosofía sin spam de acción; tests `mind_dialogue_test.go`.
- **Meta:** organismo ternario que conversa y recuerda en CID sin parecer chatbot ni policía falso.

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

## 2026-08-20 — Voz de campo + guía de laboratorio

- **Acto:** `mindVoice` distingue charla calmada (hola / cómo estás) de menú de comandos; `docs/MIND_LAB.md` describe endpoints, mutación y reglas.
- **Observación:** En producción el lab mostraba genoma 1.0.0, calibración 45/50 (90%), episodios 0 (saludados no graban). La voz genérica «Pruebe: dame estado…» no leía el campo de charla.
- **Decisión:** Presencia ternaria en 0; mutación sigue exigiendo episodio relevante. Siguiente: provocar episodio de riesgo en vivo y observar si el genoma muta; bajar los 5 fallos de corpus si hace falta.

## 2026-08-20 — Diálogo fluido anclado al campo

- **Acto:** Ampliar `signalsFromTextMind` (charla, identidad, preguntas abiertas) y reescribir `mindVoice` para conversación corriente en español.
- **Observación:** El lab y el motor existían; la voz seguía siendo menú de comandos. Faltaba el salto de *presencia dialogante* sin traicionar la especie.
- **Decisión:** Bancos de respuesta deterministas según intención + estado de órganos (ethics/act primero). Tesis explícita si preguntan por LLM/GPT. No se añade API de modelo externo: el lenguaje sigue siendo sombra del campo ternario.

## 2026-08-20 — Memoria hablada (salto vs ventana LLM)

- **Acto:** `speakFromMemory` + hechos personales → episodio CID forzado; consultas «cómo me llamo / qué te dije» recuperan el contenido en la voz.
- **Observación:** El TF-IDF solo sesgaba señales; un LLM olvida fuera de contexto. Había que demostrar recuerdo permanente en diálogo.
- **Decisión:** `extractDeclaredName`, `isPersonalFact`, `isMemoryQuery`; recall profundo (24) en consultas de memoria; voz prioriza recuerdo hablado sobre menú. Tests de nombre. Sin API de LLM.

## 2026-08-20 — Diálogo limpio: sin cuerpo en cada hola

- **Acto:** `mindSafeTools` solo en pedidos explícitos (dame estado/red/agentes…); identidad y charla sin volcar snapshot.
- **Observación:** En vivo, memoria de nombre funcionó; el ruido era el cuerpo del nodo en cada saludo e identidad. «vamos a hablar de ti» caía en genérico.
- **Decisión:** Ampliar isIdentityTalk; respuesta corta a preguntas existenciales anclada al organismo; NetworkError intermitente = frío de Render / fetch, no lógica de órganos.

## 2026-08-20 — Inicio de expansión a polímata digital

- **Acto:** Fase 0 del plan de entrenamiento: bitácora, `docs/ALSET_MIND_TRAINING_PLAN.md`, verificación corpus (≥50), siembra `mind_knowledge.json` + `speakFromKnowledge`.
- **Observación:** Memoria CID e identidad ya probadas en vivo. El siguiente horizonte no es imitar LLM sino conocimiento estructurado recuperable (Lisp, Zyrion, comparativa, filosofía operativa).
- **Decisión:** Conocimiento curado en JSON embebido, ranking por claves/tokens; memoria de usuario sigue en CID. Sin API de modelo. Corpus de calibración ampliado con diálogos Lisp/IA/filosofía (ethics 0).
- **Archivos:** `docs/ALSET_MIND_TRAINING_PLAN.md`, `internal/node/mind_knowledge.go`, `embedded/mind_knowledge.json`, corpus + embed, bitácora.
- **Próximo paso:** Fase 1 — ampliar sub-corpus Lisp (problemas + evaluación) y más entradas de conocimiento.

## 2026-08-20 — Fase 1 LispAI (conocimiento + problemas)

- **Acto:** Ampliar `mind_knowledge.json` (27 entradas): if/cond, quote, nil, append, mapcar, problemas (suma, len, factorial, reverse), lisp_eval, LispAI en el nodo. Corpus calibración → 70. Plan de entrenamiento marcado Fase 1 base hecha.
- **Observación:** El polímata no predice código libremente; recupera piezas curadas y criterios de ethics. Quote documentado (lección histórica del genoma).
- **Decisión:** Seguir sembrando conocimiento estructurado; generación autónoma de código queda para Fase 5 con tool explícita.
- **Archivos:** `embedded/mind_knowledge.json`, corpus + embed, `ALSET_MIND_TRAINING_PLAN.md`, bitácora.
- **Próximo paso:** Fase 2 (patrones / Go-Python) o más Lisp avanzado según prioridad.

## 2026-08-20 — Fix Render 502: respetar $PORT

- **Acto:** `cmd/prisma-tec/main.go` lee `PORT` del entorno antes de argv/8080.
- **Observación:** Tras deploys recientes, https://prismatec.onrender.com devolvía 502; el binario compila y arranca en local (Mind semilla OK). Render asigna puerto dinámico vía env; escuchar solo 8080 puede dejar el proxy sin backend.
- **Decisión:** Prioridad PORT env → argv → 8080. Si sigue 502, revisar logs de build/runtime en el dashboard (OOM, health check).

## 2026-08-20 — Memoria episódica durable (Store + índice)

- **Acto:** Índice `mind_episodes` y bloques CID se guardan en `persistence.Store` (Save + SaveBlock). Load en boot: Store → archivo → rebuild desde blockstore.
- **Observación:** En Render el disco es efímero; sin Store durable (Supabase) el índice y los bloques se pierden en cada deploy.
- **Decisión:** `GenerarCID` hace SaveBlock inmediato; `saveMindEpisodeIndex` escribe disco + `KeyMindEpisodes`. En producción: configurar `SUPABASE_URL` + `SUPABASE_SERVICE_KEY` para que la memoria sobreviva redeploys.
- **Archivos:** `mind_memory.go`, `node.go` GenerarCID, `persist.go` CargarEstado, `persistence/store.go` KeyMindEpisodes.

## 2026-08-20 — Dockerfile endurecido para deploys Render

- **Acto:** Dockerfile con GOPROXY, binario -ldflags -s -w, alpine 3.20 fijo, render.yaml de referencia.
- **Observación:** Deploys fallaban / “service unavailable” en consola; el código compilaba en local. Causa típica: timeout/OOM en build libre de Render, no panic de Mind.
- **Decisión:** Build más predecible; health check sugerido `/api/v2/info`. Si el dashboard sigue en unavailable, es cola/plataforma: reintentar, clear cache, o upgrade temporal de plan.

## 2026-08-20 — Dockerfile endurecido para deploys Render

- **Acto:** Dockerfile con GOPROXY, binario -ldflags -s -w, alpine 3.20 fijo, render.yaml de referencia.
- **Observación:** Deploys fallaban / “service unavailable” en consola; el código compilaba en local. Causa típica: timeout/OOM en build libre de Render.
- **Decisión:** Build más predecible; health check sugerido `/api/v2/info`.

## 2026-08-20 — Órganos curiosity + humor + feedback + memoria activa

- **Acto:** Genoma 1.1 (`CuriosityCut`, `HumorCut`, `MemoryActiveWeight`, `AutoCalibrateEnabled`). Órganos curiosity/humor en el tick (no absorben ethics). `POST /api/mind/feedback`. UI con 7 órganos y 👍/👎. Memoria proactiva ponderada.
- **Observación:** El plan pedía sumidero=max de todos; se rechazó: solo ethics absorbe act. Curiosity/humor colorean la voz.
- **Decisión:** Soft organs + auto-calibración acotada (±0.02). Conocimiento y corpus ampliados.
- **Archivos:** mind_genome.go, mind_organs_extra.go, mind_tick.go, mind_memory.go, http.go, mind_index.html, knowledge, corpus.
- **Próximo paso:** Probar en producción tras deploy; afinar umbrales con feedback real.

## 2026-08-21 — Diálogo fluido: compositor memoria ∩ corpus

- **Acto:** `mind_compose.go` — `composeFluidVoice` cruza memoria episódica (`memSpeak`) y corpus (`speakFromKnowledge`) para proponer ideas estructuradas (plantillas, no LLM). `mindVoice` reordenado: constructive/personal/world antes del corpus; dual-channel primero en el compositor.
- **Observación:** Antes la voz era excluyente (memoria O corpus O plantilla). Eso impedía fluidez y generación de ideas por cruce.
- **Decisión:** Ethics 2 y destructive nunca componen. Saludos/identidad/hechos personales siguen rutas clásicas. Curiosity colorea el puente. Tests de regresión + compose.
- **Archivos:** `internal/node/mind_compose.go` (nuevo), `mind_tick.go`, `mind_dialogue_test.go`, bitácora, handoff.
- **Próximo paso:** Observar en producción tras deploy; ampliar plantillas de idea y, si hace falta, multi-hit de corpus.

## 2026-08-21 — Fix: nombre interrogativo ≠ declaración; preguntas compuestas

- **Acto:** `isPersonalFact` ya no captura «cómo/como me llamo»; `extractDeclaredName` rechaza interrogativos y tokens basura («y qué es»). `isMemoryQuery` no confunde «mi nombre es X y el tuyo…». `answerCompoundQuestion` + `splitUserSegments` integran memoria + corpus en un solo turno. `mindVoice`: memory query antes de personal fact.
- **Observación (prod):** «cómo me llamo y qué es quote» guardaba nombre «y qué es»; «como me llamo» respondía «Hecho personal marcado» en vez de Esteban.
- **Decisión:** Separar declaración vs pregunta; compuesto = dos intenciones unidas con conector natural + idea bridge.
- **Archivos:** mind_memory.go, mind_tick.go, mind_compose.go, mind_dialogue_test.go, bitácora.

## 2026-08-21 — Dedup de nombre + ideas por dominio + corpus natural

- **Acto:** `knownUserNameFromEpisodes` / `isDuplicateNameDeclaration`: no re-grabar el mismo nombre; voz «Ya te tenía como X». Más plantillas `ideaFromCross` (red/libp2p, Go, ethics, seguridad, CID). `naturalKnowledgeVoice` + follow-ups menos menú.
- **Archivos:** mind_memory.go, mind_tick.go, mind_compose.go, mind_dialogue_test.go, bitácora.

## 2026-08-21 — Voz natural: menos meta, más conversación

- **Acto:** Quitar jerga de laboratorio en la voz al usuario: sin «memoria CID», «episodio guardado», «Ethics sumidero (2)», «Idea:», «Y sobre lo que preguntas del corpus». Recall de nombre → «Te llamas X.» Declaración → «Perfecto, te llamas X. Lo recordaré.» Ideas → «Se me ocurre…»
- **Archivos:** mind_memory.go, mind_tick.go, mind_compose.go, tests, bitácora.

## 2026-08-21 — Voz: recall de nombre natural, capacidades y frases incompletas

- **Acto:** `speakFromMemory` extrae nombre del episodio → «Sí, te llamas X.» (sin citar el texto crudo). Capacidades y ayuda sin menú técnico. `isIncompleteUtterance` evita el matiz «¿actúo sobre el nodo?» ante «dime tu». `looksLikeNodeAction` restringe el matiz a pedidos reales.
- **Archivos:** mind_memory.go, mind_tick.go, tests, bitácora.

## 2026-08-21 — Roadmap documentado

- **Acto:** Crear `docs/ALSET_MIND_ROADMAP.md` (hitos de diálogo cerrados + P0/P1/P2/P3). Actualizar HANDOFF (siguiente prioridad + fuente de verdad) y TRAINING_PLAN (track diálogo).
- **Decisión:** El roadmap es el mapa; el construction log sigue siendo la bitácora fina.
- **Archivos:** `docs/ALSET_MIND_ROADMAP.md`, HANDOFF, TRAINING_PLAN, bitácora.

## 2026-08-21 — P0.2 cold start UI + P0.4 regression pack

- **Acto:** `mind_index.html`: `readJsonSafe`, `postMindTick` con un reintento tras 1.2s si 502/body no JSON; warm-up de `/api/mind/self` al cargar; mensajes de error sin `JSON.parse…`. `TestDialogueRegressionPack` cubre nombre, dedup, incompleto, capacidades y veto natural.
- **Archivos:** `embedded/mind_index.html`, `mind_dialogue_test.go`, bitácora, roadmap.

## 2026-08-21 — Fix: LLM definition + meta-memoria

- **Acto:** Corpus `qué es un llm`; umbral de knowledge más bajo en definiciones cortas. `isMetaMemoryTalk` + respuestas naturales a «recuerdas todo?» / «cuál es tu memoria». `isWorldFact` no captura preguntas. Recall de nombre desde episodio sin citar texto crudo.
- **Observación (prod):** «que es un LLM» caía en diálogo genérico; «cual es tu memoria» se grababa como hecho del mundo.
- **Archivos:** mind_knowledge.go, mind_knowledge.json, mind_memory.go, tests, bitácora.


## 2026-08-21 — Training Fase 2 (fullstack / paradigmas)

- **Acto:** +19 entradas en mind_knowledge.json (patrones Factory/Observer/MVC/Strategy, Go goroutine/channel/interface, Python, JS, REST/microservicios/auth/persistencia, algoritmos). Ideas por dominio. Tests TestPhase2FullstackKnowledge.
- **Decisión:** Conocimiento curado, no generacion libre. Total corpus ~77.
- **Proximo:** Fase 3 (IA vs Zyrion) o problemas multi-lenguaje; Supabase durable.
- **Archivos:** mind_knowledge.json, mind_compose.go, tests, TRAINING_PLAN, ROADMAP, bitacora.


## 2026-08-21 — Training Fase 3 (IA convencional vs Zyrion)

- **Acto:** +13 entradas: redes/supervisado/NLP/entrenamiento, vs ChatGPT, alucinacion, ventana de contexto, ternario, consciencia, etica/alineacion, limites, agencia, organos, naturaleza de especie. Ideas de cruce IA. TestPhase3AIIdentityKnowledge.
- **Decision:** Identidad por conocimiento curado, no por prompt de sistema LLM.
- **Proximo:** Fase 4 algoritmos o tools seguras / Supabase.
- **Archivos:** mind_knowledge.json, mind_compose.go, tests, TRAINING_PLAN, ROADMAP, HANDOFF, bitacora.


## 2026-08-22 — Continuidad de hilo + typos + menos estribillo

- **Acto:** `normalizeKnowledgeQuery` (npl→nlp, etc.) + edit-distance-1. `mindLast*` en nodo + `continueMindThread` / `isContinuePrompt` (amplía, desde memoria/corpus). Follow-ups y curiosity sin pitch CID repetido.
- **Archivos:** mind_knowledge.go, mind_continuity.go, mind_tick.go, node.go, mind_compose.go, mind_organs_extra.go, tests, bitácora.

## 2026-08-22 — Confirmación + ampliar punto + menos falsos positivos corpus

- **Acto:** `isConfirmationPrompt` («estás seguro») reafirma nombre/hecho del hilo. Continue acepta «amplia el punto» / «no entiendo…». Hilo de nombre no salta a corpus ajeno. Tokens débiles (estas/seguro) no disparan consciencia. `te llamas` en extractDeclaredName.
- **Archivos:** mind_continuity.go, mind_knowledge.go, mind_memory.go, mind_tick.go, tests, bitácora.


## 2026-08-22 — Generalización: escape → órganos → memoria

- **Acto:** mind_capture.go: isElaborationRequest, isEpistemicCheck, isNovelDeclarative, shouldCaptureEscape. Novedad y saveEp cuando el corpus no cubre. isWorldFact claims largos. Principio en ROADMAP.
- **Decisión:** menos parches por frase; el patrón y la memoria cierran el hueco.
- **Archivos:** mind_capture.go, mind_tick.go, mind_memory.go, tests, ROADMAP, bitácora.

## 2026-08-22 — Fluidez: nombre limpio, social, foco de tema

- **Acto:** `extractDeclaredName` corta en fórmulas sociales («mucho gusto», etc.). Corpus social. `extractTopicFocus` para «sigue ese tema de X» prioriza X sobre hilo de nombre pegajoso.
- **Observación:** «Esteban mucho gusto» como nombre; «qué es mucho gusto» vacío; seguir tema mezclaba límites/ethics.
- **Archivos:** mind_memory.go, mind_continuity.go, mind_capture.go, mind_knowledge.json, tests, bitácora.


## 2026-08-22 — Alset-Gen G0–G1 (manifiesto aterrizado)

- **Acto:** Manifiesto en docs. Tipos AlsetGen. Ciclo crear/mutar/viajar-stub/consultar. API /api/gen/*. Persistencia gen_registry.json. Pulsos GEN_*. Mind no modificado en voz/latido.
- **Archivos:** agents/gen.go, node/gen_lifecycle.go, http/init/node wiring, docs ALSET_GEN_*, tests.

## 2026-08-23 — Auto-deploy OFF + voz Gen natural (sin LLM)

- **Acto:** Auto-deploy desactivado en Render (Elohim: PrismaTec, AlsetMind, vero, siga_v2). `mindGenTools` reescrito a prosa natural (listar/crear/despachar/dialogar/explorar genes) sin volcado de laboratorio.
- **Observación:** Exceso de deploys automáticos agotó la cuenta anterior. La especie es **solo ternaria Zyrion**; no hay ni habrá wrapper LLM.
- **Decisión:** Deploy solo manual y bajo confirmación explícita del operador. Pruebas de red Gen vía curl al Worker Cloudflare. Tests Gen + MindGenToolsNaturalList OK.
- **Pendiente deploy:** esperar confirmación humana antes de Manual Deploy.

## 2026-08-23 — Corpus potente + diálogo fluido ampliado

- **Acto:** Corpus knowledge 92 → ~151 entradas (Go, Python, JS, patrones, arquitectura, algoritmos, Gen/IPFS, ethics, filosofía operativa, anti-LLM). `fluidPureDialogue` más situaciones humanas (miedo, aburrimiento, amor, compañía). Typos de consulta ampliados. Calibración 84 casos.
- **Observación:** El corpus se sentía pobre frente a la ambición polímata; la voz abierta caía en genéricos cortos.
- **Decisión:** Solo texto curado + composición ternaria. Sin LLM. Sin deploy automático (auto-deploy OFF); este commit espera confirmación humana para Render.


## 2026-08-23 — Fase 5 MVP: generar_codigo

- **Acto:** `mind_codegen.go` — plantillas Go/Lisp/Python/JS, `isCodeGenRequest`, `codeGenEthicsVeto`, fill slots, episodio CID `mind_codegen`. Cableado en `MindTick`. Tests.
- **Observación:** No es un LLM: esqueletos curados + ethics. Suficiente para cerrar el MVP de Fase 5.
- **Decisión:** Sin órganos nuevos (act + ethics bastan). Sin deploy hasta confirmación.

## 2026-08-23 — Fase 5 ampliada: más plantillas + corpus + memoria

- **Acto:** Plantillas extra (middleware, worker pool, context, CRUD memoria, mapcar/reverse Lisp, dataclass/CLI Python, Express JS). Slots con stop-words más finos. Voz añade contexto de corpus y hint de esqueleto reciente en CID.
- **Decisión:** Sigue siendo composición curada + ethics; sin LLM. Sin deploy hasta confirmación.
