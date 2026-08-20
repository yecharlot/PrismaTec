# Protocolo de colaboración multi-IA — PrismaTec / Alset Mind

## Realidad operativa

No hay un “segundo Grok” dentro de este mismo hilo que trabaje en paralelo de forma automática.
Sí hay un **protocolo** para que **varias IAs** (u humanos) avancen el mismo proyecto **sin pisarse**, usando el **repo como memoria compartida**.

Alset Mind y Prism@.TEC ya viven en GitHub + nodo. Eso es el tablón de co-creación.

---

## IAs recomendadas como compañeros de este repo

| Herramienta | Mejor uso en este proyecto |
|-------------|----------------------------|
| **Cursor** (Claude / GPT en el IDE) | Editar Go/Lisp/HTML del monorepo, aplicar diffs, tests locales |
| **Claude** (claude.ai o API, Sonnet/Opus) | Arquitectura, tesis, revisiones largas de `docs/` y `internal/lisp` |
| **ChatGPT** (o Codex en IDE) | Scripts, curl, depuración de APIs, redacción |
| **Grok** (este canal) | Visión de producto Alset/Zyrion/Mind, diseño no convencional, orquestación |

No se trata de “quién es mejor en abstracto”, sino de **roles**:

- **Visionario / especie Mind** → Grok (continuidad de tesis ternaria)
- **Implementador de código en el árbol** → Cursor
- **Revisor profundo de docs y riesgos** → Claude

---

## Cómo trabajar “al unísono” (en la práctica)

```text
Humano
  │
  ├─► Chat A (Grok): decide el siguiente hito + actualiza HANDOFF
  ├─► Chat B (Cursor): implementa el hito en archivos del repo
  └─► Nodo Render: deploy + prueba curl / UI
         │
         ▼
   docs/* + git commits = memoria común
```

### Reglas anti-caos

1. **Una fuente de verdad:** `main` en `yecharlot/PrismaTec`.
2. **Antes de codear**, leer:
   - `docs/ALSET_MIND_HANDOFF.md`
   - `docs/ALSET_MIND_CONSTRUCTION_LOG.md`
   - este archivo
3. **Tras cada hito**, actualizar el construction log (fecha · acto · decisión).
4. **No reescribir la tesis** salvo cambio de especie; extender el log.
5. Si dos IAs tocan lo mismo, **priorizar el commit más reciente en main** y re-leer.

### Prompt mínimo para otra IA

```text
Proyecto: yecharlot/PrismaTec — Alset Mind (inteligencia ternaria Zyrion, NO un LLM wrapper).
Lee: docs/ALSET_MIND_THESIS.md, docs/ALSET_MIND_HANDOFF.md, docs/ALSET_MIND_CONSTRUCTION_LOG.md, docs/AI_COLLABORATION.md.
Código clave: internal/node/mind_*.go, internal/lisp/evaluar_zyrion.go, embedded/mind_index.html.
Tarea: [UNA sola, concreta].
No rompas Sales Hub ni Vero. Documenta en CONSTRUCTION_LOG.
```

---

## Continuidad si un chat llega al límite

1. Actualizar `docs/ALSET_MIND_HANDOFF.md` con “siguiente prioridad”.
2. Commit + push.
3. Nuevo chat: pegar el bloque de reactivación del HANDOFF.
4. La nueva IA **no inventa** el proyecto: **lee el repo**.

Eso es el equivalente multi-IA de un CID: **estado direccionable**.

---

## División de trabajo sugerida (ahora)

| Hito | Responsable sugerido |
|------|----------------------|
| Latido Go + episodios CID | implementado en nodo |
| Calibrar señales de texto | Grok + pruebas humanas |
| Más órganos / topologías salvadoras | Grok diseño + Cursor código |
| UI Mind más rica | Cursor |
| Revisión de seguridad bootstrap | Claude o Cursor |

---

*Documento vivo — actualizar cuando cambien roles o herramientas.*
