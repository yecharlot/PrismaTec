# Alset Mind — Roadmap

**Especie:** inteligencia ternaria Zyrion residente en el Nodo Alset (no LLM wrapper).  
**Repo:** `yecharlot/PrismaTec` · Cara: `https://prismatec.onrender.com/w/mind.app.ans`  
**Actualizado:** 2026-08-21

Este documento es el mapa de avance. La bitácora detallada sigue en `ALSET_MIND_CONSTRUCTION_LOG.md`. La tesis no se reescribe aquí.

---

## Estado actual (resumen)

| Capa | Estado |
|------|--------|
| Latido nativo Go + 7 órganos | Operativo |
| Memoria episódica CID + bias + dedup de nombre | Operativo |
| Corpus curado + `speakFromKnowledge` | Operativo (Fase 2 fullstack sembrada) |
| Compositor mem ∩ corpus → ideas | Operativo |
| Voz natural (sin jerga de laboratorio) | Operativo (iteración 2026-08-21) |
| UI laboratorio `/w/mind.app.ans` | Operativo |
| Training plan Fase 0–1 (Lisp base) | Hecho |
| Tools de acción seguras bajo ethics | Pendiente |
| Mutación de topología (más allá de umbrales) | Pendiente |
| Enjambre / memoria entre peers | Base de pulsos; no cerrado |

---

## Hitos cerrados (diálogo 2026-08-21)

Orden aproximado de entrega en el día:

1. **Compositor fluido** (`mind_compose.go`) — cruce memoria episódica + corpus con plantillas de idea (no predicción de tokens).
2. **Declaración ≠ pregunta de nombre** — `isPersonalFact` / `extractDeclaredName` / `isMemoryQuery` corregidos; sin guardar «y qué es» como nombre.
3. **Preguntas compuestas** — `splitUserSegments` + `answerCompoundQuestion` (nombre + quote en un turno).
4. **Dedup de nombre** — no re-grabar el mismo nombre; voz «ya te conocía como X».
5. **Ideas por dominio** — Lisp, red/libp2p, Go, ethics, seguridad, CID.
6. **Corpus más natural** — `naturalKnowledgeVoice`, follow-ups sin menú de comandos.
7. **Voz sin meta-jerga** — fuera de la boca del usuario: «memoria CID», «Ethics sumidero (2)», «Idea:», «episodio guardado».
8. **Recall natural del nombre** — «Sí, te llamas X.» sin citar el texto crudo del episodio.
9. **Frases incompletas** — «dime tu» pide completar; no dispara matiz de acción sobre el nodo.
10. **Capacidades en lenguaje humano** — sin listar endpoints ni jerga interna.

Código clave: `internal/node/mind_tick.go`, `mind_compose.go`, `mind_memory.go`, `mind_knowledge.go`, `mind_organs_extra.go`.

---

## Roadmap próximo

### P0 — Estabilizar en producción (corto plazo)

| # | Ítem | Criterio de hecho |
|---|------|-------------------|
| P0.1 | Observar compositor + voz natural en Render | Diálogos reales sin regresión de ethics/veto |
| P0.2 | Cold start JSON en primer `hola` | **Hecho 2026-08-21** — retry + warm-up en `mind_index.html` |
| P0.3 | Supabase para episodios durables | `GET /api/mind/memory` sobrevive redeploy |
| P0.4 | Suite de regresión de diálogo ampliada | **Parcial** — `TestDialogueRegressionPack` (+ tests previos del día) |

### P1 — Diálogo y polímata (medio plazo)

| # | Ítem | Notas |
|---|------|--------|
| P1.1 | Training **Fase 2** — patrones / Go / Python | **Hecho 2026-08-21** — +19 entradas corpus (77 total) |
| P1.2 | Training **Fase 3** — IA convencional vs Zyrion | **Hecho 2026-08-21** — +13 entradas (~90 total) |
| P1.3 | Más entradas de corpus + plantillas «Se me ocurre…» | Solo conocimiento curado; sin API LLM |
| P1.4 | Multi-hit de corpus (2 entradas rankeadas) | Composición más rica sin alucinar |
| P1.5 | UI: ocultar o colapsar `note` técnico del latido | La cara puede mostrar menos “lab” al usuario final |

### P2 — Cuerpo del nodo (medio plazo)

| # | Ítem | Notas |
|---|------|--------|
| P2.1 | Tools de **acción** seguras bajo ethics/act | Crear/registrar solo con permiso claro |
| P2.2 | Confirmación ternaria (act=1) en lenguaje natural | Sin «Lo leo en matiz» |
| P2.3 | Lectura de red/peers/agentes ya existente | Mantener solo lectura por defecto |

### P3 — Evolución del organismo (largo plazo)

| # | Ítem | Notas |
|---|------|--------|
| P3.1 | Mutación de **topología** acotada (no solo umbrales) | Siempre bajo ethics |
| P3.2 | Training **Fase 4–5** — algoritmos; `generar_codigo` con tool explícita | Nunca generación libre sin veto |
| P3.3 | Enjambre: memoria compartida por CID entre peers | Pulsos `mind_episode` como base |
| P3.4 | HyperIA / cara rica reutilizando mismos endpoints | Motor sin cambios de especie |

---

## Principios que no se negocian

1. **No es un LLM wrapper** — el lenguaje es sombra del campo ternario.
2. **Ethics 2 absorbe act** — lo destructivo no se ejecuta desde el diálogo.
3. **Memoria content-addressed** — hechos del usuario en CID / Store, no solo ventana de chat.
4. **Mutación solo si mejora score de calibración** (o política explícita futura igualmente acotada).
5. **Una fuente de verdad:** `main` en `yecharlot/PrismaTec` + docs de handoff/log/roadmap.

---

## Orden de trabajo sugerido (multi-IA)

Ver `docs/AI_COLLABORATION.md`.

| Rol | Enfoque actual |
|-----|----------------|
| Grok | Priorizar hitos del roadmap; no reiniciar tesis |
| Cursor | Implementar P0/P1 en `mind_*.go` + tests |
| Claude | Revisión de riesgos ethics/bootstrap y docs largos |

Tras cada hito: actualizar **este roadmap** (checkboxes mentales / mover filas), `ALSET_MIND_CONSTRUCTION_LOG.md` y la fila «Siguiente prioridad» de `ALSET_MIND_HANDOFF.md`.

---

## Enlaces

| Documento | Rol |
|-----------|-----|
| `ALSET_MIND_THESIS.md` | Qué es la especie |
| `ALSET_MIND_CONSTRUCTION_LOG.md` | Qué se hizo (bitácora) |
| `ALSET_MIND_HANDOFF.md` | Continuidad entre sesiones |
| `ALSET_MIND_TRAINING_PLAN.md` | Fases de conocimiento curado |
| `MIND_LAB.md` | Guía rápida de laboratorio / API |
| `AI_COLLABORATION.md` | Roles multi-IA |

*Documento vivo — 2026-08-21.*
