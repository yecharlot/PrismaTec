# Handoff — PrismaTec / Alset Mind + Gen

**Canónico.** Actualizado: **2026-08-29**  
**Repo:** https://github.com/yecharlot/PrismaTec · rama `main`  
**Tip de referencia al escribir esto:** `5691ff0` (`feat(mind): sessions, ternary neuron cortex, richer dialog templates`) + commits de consolidación de docs/diálogo posteriores.

Para operar día a día: [GUIA.md](GUIA.md) · Mapa del repo: [../README.md](../README.md)

---

## 1. Especie (no negociable)

| | Alset Mind | LLM típico |
|--|------------|------------|
| Motor | Zyrion 0/1/2 + órganos | Probabilidades de tokens |
| Memoria | Episodios CID (+ sesión) | Ventana de contexto |
| Ética | Órgano ethics explícito | Aprendida / opaca |
| Gen | Sondas orquestadas | N/A |

---

## 2. Dónde estamos (nivel)

**Fase:** organismo operativo + **confianza de diálogo en consolidación** (sesiones, director, razón, scout, córtex ternario semilla).

| Área | Estado |
|------|--------|
| 7 órganos + genoma | Sí |
| Sesiones (`session`) aislamiento memoria | Sí — verificado (Diego s-a vs s-b) |
| Director / dialog flow / topic stack | Sí |
| Razón ternaria (silogismos) | Sí (MVP fuerte) |
| Scout web vía gen + learn | Sí |
| Gen: create / explore / dispatch CF / return / delete / memory | Sí |
| Codegen plantillas + ethics | MVP |
| Córtex `ternaryNeuron` | Semilla (`mind_ternary_net.go`) |
| Corpus knowledge | ~346+ entradas (en crecimiento) |
| Store Cloudflare DO | Código + worker desplegable |
| Docs unificadas | README + HANDOFF + GUIA |

### Producción (referencia)

| Pieza | URL / nota |
|-------|------------|
| Nodo Render (Elohim) | `https://prismatec-4u5c.onrender.com` (auto-deploy OFF) |
| Edge | `https://alset-network.lhmolam-877.workers.dev` |
| Lab | `/w/mind.app.ans` |

Confirmar siempre el commit desplegado: puede ir detrás de `main` local.

---

## 3. A dónde vamos

1. **Diálogo profundo estable** en todos los dominios del corpus (menos “bordes” de enrutado).  
2. Córtex ternario con más rutas útiles (sin volverse red neuronal estadística).  
3. Codegen por composición de fragmentos.  
4. Memoria de red Mind↔Gen↔CF coherente en el hábito diario.  
5. Handoff siempre = verdad del tip de `main`.

---

## 4. Qué falta / riesgos

| Hueco | Notas |
|-------|--------|
| Diálogo aún tiene bordes | Consolidación en curso (templates + corpus + gates) |
| STATUS docs viejos | Reemplazados por **este** HANDOFF |
| Prod ≠ main | Deploy manual bajo confirmación |
| Scout | Fragmentos web, no comprensión total de internet |
| Mutación topología libre | Fuera de alcance a propósito |

---

## 5. Reglas de oro

1. Nada de wrapper LLM como motor de Mind.  
2. Ethics es sumidero real.  
3. No romper módulos ajenos (Sales, Vero, etc.) sin necesidad.  
4. Documentar solo en README / HANDOFF / GUIA (+ log opcional).  
5. Tests de diálogo: `go test ./internal/node/ -run Dialogue` y `scripts/mind_dialogue_battery.py`.

---

## 6. Archivo histórico

Los archivos `docs/ALSET_MIND_HANDOFF.md`, `ALSET_STATUS_HANDOFF.md`, roadmaps sueltos, etc. son **legado**. Si contradicen este HANDOFF, **gana este archivo**.

Bitácora cronológica opcional: `ALSET_MIND_CONSTRUCTION_LOG.md` (append-only).
