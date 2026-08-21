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

- [ ] Patrones: Singleton, Factory, Observer, MVC (ejemplos multi-lenguaje).
- [ ] Lenguajes: Go, Python, JS — sintaxis, errores, concurrencia.
- [ ] Arquitectura: REST, microservicios, auth, datos.

## Fase 3 — IA convencional vs naturaleza ternaria

- [ ] Conceptos: redes, supervisado/no supervisado, NLP.
- [ ] Comparativa LLM vs Zyrion (identidad reforzada).
- [ ] Filosofía de la IA: consciencia, ética, límites.

## Fase 4 — Algoritmos y problemas

- [ ] Ordenación, búsqueda, recursión, programación dinámica.
- [ ] Complejidad: O(n), O(log n), optimización.
- [ ] Problemas fullstack integrados.

## Fase 5 — Generación de código (horizonte)

- [ ] Tool explícita `generar_codigo` (no charla libre sin ethics).
- [ ] Composición desde memoria CID / conocimiento conocido.
- [ ] Evaluación ternaria del código generado (ethics veta destructivo).
- [ ] Cada generación → episodio CID (solicitud + código + evaluación).

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

**Siguiente conocimiento:** retomar **Fase 2** (patrones / Go-Python) según `docs/ALSET_MIND_ROADMAP.md` § P1.

