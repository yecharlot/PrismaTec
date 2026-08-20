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

