# Alset Mind — Protocolo de continuidad (handoff)

Este archivo existe para que la co-creación **no muera** cuando un chat, un modelo o una sesión alcance su límite.

## Cómo seguir juntos aunque el hilo se corte

1. **Fuente de verdad en el repo** (no en la memoria del chat):
   - `docs/ALSET_MIND_THESIS.md` — qué es la especie
   - `docs/ALSET_MIND_CONSTRUCTION_LOG.md` — qué se hizo y qué falta
   - `docs/ALSET_MIND_HANDOFF.md` — este puente
   - `docs/ALSET_MIND_ROADMAP.md` — roadmap de hitos
   - `internal/node/mind_bootstrap.go` — genoma vivo
   - `internal/node/embedded/mind_index.html` — cara del organismo

2. **Al abrir un chat nuevo**, el humano pega o dice:
   > Continúa Alset Mind desde el repo PrismaTec. Lee `docs/ALSET_MIND_HANDOFF.md` y `docs/ALSET_MIND_CONSTRUCTION_LOG.md`. No reinicies la tesis; avanza el siguiente ítem pendiente.

3. **Estado operativo actual (actualizar tras cada hito)**

| Campo | Valor |
|-------|--------|
| Commit semilla | buscar `feat(mind)` en main |
| URL cara | `https://prismatec.onrender.com/w/mind.app.ans` |
| Agente | `mind.alset.ans` / id `mind-alset` |
| Genoma Lisp | `mind-latido`, `mind-eval-organ` |
| Bug conocido (histórico) | topología sin `quote` → "faltan :entradas o :salidas" — corregido con quote |
| API latido | POST /api/mind/tick · GET /api/mind/self |
| Memoria | mind_episodes.json + bias de vetos + pulse mind_episode |
| Siguiente prioridad | ver `docs/ALSET_MIND_ROADMAP.md` — P0 prod (cold start, Supabase, regresión); P1 Fase 4 o P0.3 Supabase / P2 tools seguras |

4. **Regla de oro**  
   Si el asistente “olvida”, **no se improvisar de cero**: se lee el repo.  
   Alset Mind es content-addressed y document-addressed a la vez.

5. **Mensaje mínimo de reactivación** (copiar/pegar)

```text
Proyecto: Alset Mind en yecharlot/PrismaTec.
Lee docs/ALSET_MIND_THESIS.md, CONSTRUCTION_LOG.md, HANDOFF.md, ROADMAP.md.
Estado: organismo con mind-latido + UI /w/mind.app.ans.
Siguiente: [escribe aquí la tarea].
No trates Mind como un LLM wrapper; especie ternaria Zyrion.
```

Última actualización: 2026-08-21 — Training Fase 3 (IA vs Zyrion); corpus ~90; voz natural.


## DeepSeek / contexto largo

Usar `docs/DEEPSEEK_HANDOFF.md` (prompt de arranque listo para pegar).
