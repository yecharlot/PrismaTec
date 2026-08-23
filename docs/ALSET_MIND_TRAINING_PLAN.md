# Alset Mind — Plan de entrenamiento (polímata digital)

**Especie:** organismo ternario Zyrion, no LLM.  
**Principio:** conocimiento estructurado vía corpus, episodios CID y evaluación de órganos — no predecir tokens.

---

## Reglas de oro

1. No convertir Mind en wrapper de API de LLM.
2. No romper Sales Hub, Vero ni panel admin.
3. Documentar cada hito en `docs/ALSET_MIND_CONSTRUCTION_LOG.md`.
4. No reescribir la tesis; solo expandirla.
5. Mutaciones acotadas y reversibles; ethics tiene veto.

---

## Fase 0 — Preparación ✅

- [x] Bitácora: inicio de expansión a polímata.
- [x] Este documento maestro.
- [x] Corpus base ≥ 50 diálogos (`docs/mind_calibration_dialogs.json`).
- [x] Base de conocimiento sembrable (`mind_knowledge.json`) recuperable por overlap.

## Fase 1 — Dominio LispAI (lengua materna)

- [x] Sub-corpus Lisp: `defun`, `let`, `if`/`cond`, `lambda`, `car`/`cdr`/`cons`, `quote`, `nil`, `append`, `mapcar`, recursión (`mind_knowledge.json`).
- [x] Problemas Lisp con solución: suma lista, length, factorial, reverse.
- [x] Evaluación de código Lisp bajo Zyrion (entrada `lisp_eval`: forma + ethics).
- [x] Voz: `speakFromKnowledge` recupera entradas Lisp/problemas.
- [ ] Ampliar (macros, closures avanzados, más problemas) en iteraciones siguientes.

## Fase 2 — Fullstack y paradigmas

- [x] Patrones: Singleton, Factory, Observer, MVC, Strategy (ejemplos ligados al nodo).
- [x] Lenguajes: Go (goroutines, channels, interfaces), Python (GIL, decorators, comprehensions), JS (event loop, async).
- [x] Arquitectura: REST, microservicios, auth, persistencia SQL/KV vs CID.
- [ ] Profundizar con problemas guiados multi-lenguaje (iteración siguiente).

## Fase 3 — IA convencional vs naturaleza ternaria

- [x] Conceptos: redes neuronales, supervisado/no supervisado, NLP, entrenamiento vs corpus.
- [x] Comparativa LLM vs Zyrion (alucinación, ventana de contexto, ternario, vs ChatGPT).
- [x] Filosofía: consciencia (sin afirmarla), ética/alineación operativa, límites, agencia.
- [ ] Iterar con más diálogos de calibración identity-focused.

## Fase 4 — Algoritmos y problemas

- [ ] Ordenación, búsqueda, recursión, programación dinámica.
- [ ] Complejidad: O(n), O(log n), optimización.
- [ ] Problemas fullstack integrados.

## Fase 5 — Generación de código (horizonte)

- [x] Tool explícita `generar_codigo` (plantillas + ethics; 2026-08-23).
- [x] Composición desde plantillas curadas + slots (ampliar con CID/corpus en iteraciones).
- [x] Evaluación ternaria: ethics 2 / patrones peligrosos → veto (sin entregar artefacto).
- [x] Cada generación → episodio `mind_codegen` CID (solicitud + código + ethics/veto).

---

## Mecánica de aprendizaje (cómo “estudia”)

| Canal | Rol |
|-------|-----|
| `mind_calibration_dialogs.json` | Ground truth de órganos (0/1/2); mutación solo si mejora score |
| `mind_knowledge.json` | Hechos y explicaciones recuperables (overlap + tipo) |
| Episodios CID | Hechos del usuario y del propio diálogo |
| Mutación de genoma | Umbrales; no rewiring libre |

## Formato de entrada en bitácora (por fase)

```markdown
## YYYY-MM-DD — [Nombre de la fase]
- **Acto:** …
- **Observación:** …
- **Decisión:** …
- **Archivos modificados:** …
- **Próximo paso:** …
```

---

*Actualizar checkboxes al cerrar cada hito. No saltar a Fase 5 sin base sólida en 1–3.*

---

## Track paralelo: diálogo natural (2026-08-21)

Completado como base para el polímata (no sustituye las fases de corpus):

- [x] Compositor memoria ∩ corpus
- [x] Declaración vs pregunta de nombre; preguntas compuestas
- [x] Dedup de nombre conocido
- [x] Voz sin jerga de laboratorio
- [x] Recall «Sí, te llamas X.»
- [x] Frases incompletas sin matiz de acción

**Siguiente conocimiento:** **Fase 4** (algoritmos/problemas) o P2 tools seguras / P0.3 Supabase. Ver ROADMAP.

