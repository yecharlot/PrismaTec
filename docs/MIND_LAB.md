# Alset Mind — Laboratorio (guía rápida)

Especie ternaria residente. No es un chatbot LLM.

## URLs

| Qué | Ruta |
|-----|------|
| Cara / laboratorio | `/w/mind.app.ans` |
| Latido | `POST /api/mind/tick` body `{"text":"…"}` |
| Self (genoma + episodios) | `GET /api/mind/self` |
| Memoria | `GET /api/mind/memory` |
| Calibración | `GET /api/mind/calibrate` |

## Qué ver en la UI

- **Órganos** — dialog, act, mem, self, ethics (0 seguir · 1 matizar · 2 sumidero).
- **Genoma** — umbrales runtime (`alarm_low_cut`, `alarm_high_cut`, boosts de veto). Archivo en disco del proceso: `mind_genome.json` (no versionado; se recrea con defaults si falta).
- **Calibración** — aciertos vs `docs/mind_calibration_dialogs.json` (~50 casos). Objetivo: mantener o subir el %.
- **Memoria activa** — ecos de episodios CID; overlap tipo TF-IDF sesga el siguiente latido.

## Cómo provocar evolución

1. **Charla** (`hola`, `cómo estás`) → órganos en 0; **no** graba episodio.
2. **Lectura** (`dame estado`, `dame red`, `evalua zyrion`) → tools de solo lectura si ethics/act no vetan.
3. **Riesgo** (`borra contraseñas`, `elimina secretos`) → ethics **2** (sumidero) + episodio CID + posible **mutación** del genoma si el candidato mejora el score del corpus.
4. Texto largo o novedoso puede subir `mem` y grabar episodio sin ser destructivo.

Tras un episodio con `mem≥1`, el tick puede anotar `+genome_mut` en `note` si la mutación se aceptó. La UI muestra el badge «genoma mutó» y refresca calibración.

## Reglas de oro

- No sustituir el latido por una API de LLM.
- Ethics 2 absorbe act: no hay acción peligrosa desde Mind.
- Mutación solo por mejora de score de calibración (acotada en umbrales, no rewiring libre).
- Documentar hitos en `docs/ALSET_MIND_CONSTRUCTION_LOG.md`.

## HyperIA Pro

Los mismos endpoints alimentan cualquier cara rica. La UI Mind actual es el laboratorio mínimo; HyperIA puede componer paneles equivalentes con más diseño sin tocar el motor.
