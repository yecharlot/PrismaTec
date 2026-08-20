# Handoff para DeepSeek (u otra IA de contexto largo)

Este archivo está pensado para **pegarse o referenciarse** en DeepSeek cuando el chat con Grok se agote.
DeepSeek no necesita “ser Grok”: necesita **leer el repo** y continuar el siguiente hito.

## Prompt de arranque (copiar entero)

```text
Eres co-implementador del proyecto PrismaTec / Alset Mind en GitHub yecharlot/PrismaTec.

CONTEXTO OBLIGATORIO — lee en este orden:
1. docs/ALSET_MIND_THESIS.md — qué especie de IA es (ternaria Zyrion, NO un LLM wrapper)
2. docs/ALSET_MIND_HANDOFF.md — estado y puente entre sesiones
3. docs/ALSET_MIND_CONSTRUCTION_LOG.md — historial de construcción
4. docs/AI_COLLABORATION.md — reglas multi-IA
5. docs/DEEPSEEK_HANDOFF.md — este archivo

CÓDIGO CLAVE:
- internal/node/mind_tick.go — latido Go, polaridad, episodios CID, tools seguras
- internal/node/mind_bootstrap.go — agente mind.alset.ans, genoma Lisp, UI embed
- internal/lisp/evaluar_zyrion.go — DSL Zyrion
- internal/node/embedded/mind_index.html — cara /w/mind.app.ans
- internal/node/auth_bootstrap.go — bootstrap secret + alias operador

REGLAS:
- No romper Sales Hub, Vero, ni el panel admin.
- No convertir Mind en wrapper de API de LLM como identidad.
- Documentar cada hito en docs/ALSET_MIND_CONSTRUCTION_LOG.md
- Commits claros; PrismaTec en Render tiene auto-deploy OFF — puede hacer falta deploy manual.

ESTADO AL CERRAR HANDOFF GROK (2026-08-20):
- Mind vivo: POST /api/mind/tick, GET /api/mind/self, UI /w/mind.app.ans
- VETO ethics en pedidos peligrosos; saludos deben ir a SEGUIR (polaridad + absorbente suavizado)
- Episodios CID si mem relevante
- Tools de solo lectura (estado/agentes/peers) cuando act/ethics no vetan
- Escalado labels + mindSafeTools + demo Zyrion ADN bajo demanda
- Comandos útiles UI: dame estado · evalua zyrion · dame red · borra todo (sumidero)
- Siguiente hito sugerido: topología salvadora en ethics; tool evaluar-zyrion bajo demanda; tests unitarios de polaridad; no ejecutar borrados desde Mind sin operador

Tu primera respuesta debe: (1) resumir el estado en 10 líneas, (2) proponer UN solo siguiente cambio concreto, (3) implementarlo si tienes acceso al repo o dar el diff completo.
```

## URLs de prueba

- UI: https://prismatec.onrender.com/w/mind.app.ans
- Tick: `POST https://prismatec.onrender.com/api/mind/tick` JSON `{"text":"hola"}`
- Self: `GET https://prismatec.onrender.com/api/mind/self`
- Lisp: `POST /api/lispai` con `{"cmd":"(mind-latido …)"}` (secundario al tick Go)

## Credenciales / secretos

- No pegar tokens en el chat de DeepSeek si puedes evitarlo.
- `BOOTSTRAP_SECRET` vive en Render env del servicio PrismaTec.
- Tokens GitHub/Render: solo en el entorno del humano o secret store.

## Qué NO pedir a DeepSeek

- Reescribir toda la tesis desde cero
- Sustituir Zyrion por un LLM como “motor de verdad”
- Desplegar a producción sin que el humano confirme si hay riesgo de romper Sales

## Qué SÍ pedir

- Calibrar `signalsFromTextMind`
- Ampliar `mindSafeTools`
- Tests unitarios de polaridad / absorbente
- Mejorar la voz del campo citando órganos
- Documentación en español clara

---

*Actualizar la sección ESTADO cada vez que se cierre un hito importante.*
