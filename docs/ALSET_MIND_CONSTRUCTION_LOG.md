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

- [ ] Calibrar umbrales con diálogos reales
- [ ] Episodios CID automáticos cuando mem=1|2
- [ ] Mutación acotada de topología bajo ethics
- [ ] Integrar cara HyperIA/laboratorio como vista opcional del mismo organismo

---

*Cada entrada nueva se añade arriba de “Pendiente” o al final con fecha.*

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

