# Análisis: silogismos como primitiva cognitiva nativa de PrismaTec

**Estado:** diagnóstico previo a implementación (sin reescritura).  
**Fecha de análisis:** 2026-10-03  
**Regla:** reutilizar → adaptar → extender; preferir camino B (80% valor, compatibilidad).

---

## 1. Qué existe hoy

### Mind (`internal/node/mind_*.go`)
- Latido `POST /api/mind/tick`: órganos ternarios (dialog, act, mem, self, ethics, curiosity, humor) + actuate.
- Voz: corpus, plantillas, memoria episódica CID, scout web, codegen acotado.
- **No es un LLM**: no predice tokens; evalúa campo 0/1/2.

### Capa de razón / silogismos (`mind_reason.go`, ~847 líneas)
Ya existe un motor **no aislado del latido**:
- Tipo `ternaryFact{ Subj, Rel, Obj, Conf int, Src string }`
- Relaciones: `es`, `es_un`, `implica`, `parte_de`, `tiene`, `no_es`, `requiere`, `causa`, `genera`, `usa`
- `Conf` en **0 / 1 / 2** (alineado a Zyrion: min-confianza en cadenas)
- Extracción gramatical de cláusulas en español (`extractClauses`, `parseRelationFact`)
- `deduceAll` / cierre transitivo (incluye conflicto → `no_es`)
- Integración en `mind_tick.go`: si `isReasoningRequest` → `reasonAboutQuery`; si no, `softReasonFromKnowledge`
- Hechos también desde episodios: `factsFromEpisodes`

### Zyrion (`internal/lisp/evaluar_zyrion.go`)
- Topologías `:entradas` / `:salidas` con estado absorbente **2**
- Usable vía LispAI; Mind lo invoca en escenarios (p.ej. checkpoint)
- No es un motor de hechos SPO; es **evaluación de campo**

### Memoria / CID
- Episodios Mind con texto + ethics + timestamp; anclaje CID
- Persistencia nodo (store / bloques); Gen con RootCID e historial

### Gene (`gen_*.go`, `agents.AlsetGen`)
- Célula ANS, RootCID, órganos snapshot, permisos, observe/explore/consult
- **No** lleva ruleset de dominio embebido como “gen cognitivo” formal, pero el concepto de **capacidades + misión + hallazgos** es el ancla natural

### Policy / Authorization
- Ethics órgano = sumidero de acción; actuate se anula si ethics=2
- Separación de facto: “decir/concluir” ≠ “ejecutar”

### LispAI
- Puede evaluar expresiones y Zyrion; no es el almacén de hechos de razón

---

## 2. Qué parte de la visión ya está implementada

| Pieza de la visión | Estado |
|--------------------|--------|
| Hechos estructurados Subj–Rel–Obj | ✅ `ternaryFact` |
| Confianza ternaria | ✅ `Conf` 0/1/2 |
| Reglas de cierre (transitividad, conflicto) | ✅ `deduceAll` |
| Voz de deducción | ✅ `formatDeductionVoice` |
| Uso en el latido | ✅ tick |
| Hechos desde memoria | ⚠️ solo parseo de texto de episodios, no grafo persistente |
| Provenance / inference_id / valid_until | ❌ ausente o implícito en `Src` string |
| Self-model por capacidad/recurso/policy | ❌ no hay API assert/ask sobre organismo |
| Invalidación al cambiar evidencia | ❌ conclusiones no versionadas ni re-evaluadas en store |
| Genes como rulesets de dominio | ❌ solo metáfora operativa |
| API app-facing assert/infer/ask/explain | ❌ |

---

## 3. Qué falta (gap crítico)

1. **Persistencia de hechos e inferencias** como objetos de primera clase (no solo texto en episodio).
2. **Reevaluación** cuando un hecho cambia (B disponible 2→0→1).
3. **Self-model operativo**: capability / resource / policy → `canExecute` derivado.
4. **Provenance** (reglas versionadas, CIDs de premisas, inference id).
5. **Separación explícita** epistemología vs autorización (hoy ethics corta acción, pero la conclusión de razón no se consulta antes de actuate de forma unificada).

---

## 4. Dónde está la capa de silogismos

**Dentro de Mind**, archivo `internal/node/mind_reason.go`, consumida por `mind_tick.go`.  
No es un paquete Core separado. Es la semilla correcta para **extender**, no para reemplazar.

---

## 5. ¿Se puede reutilizar?

**Sí.** El 80% del valor del prototipo del §22 se logra extendiendo `ternaryFact` + `deduceAll` + un **store de hechos del organismo** + un paso de re-inferencia, sin nuevo motor.

---

## 6. Representación propuesta (mínima)

Reutilizar SPO existente:

```text
ternaryFact { Subj, Rel, Obj, Conf, Src }
+ InferenceRecord {
    ID, Premises []ternaryFact, Rule string, Conclusion ternaryFact,
    EvidenceCIDs []string, RulesetVersion string, CreatedAt, ValidUntil, OrganismKey
  }
```

Estados epistemológicos mapeados al ternario actual:
- **2** = KNOWN_TRUE / evidencia a favor  
- **0** = KNOWN_FALSE / evidencia en contra  
- **1** = UNKNOWN / insuficiente (y conflicto marcado con `Rel: no_es` o flag `Conflict` si hace falta más adelante)

No introducir un cuarto valor hasta que conflicto sistemático lo exija; el código ya genera `no_es` con Conf 2 en choques.

---

## 7–9. Conexión Memory / Zyrion / Mind

- **Memory:** al anclar hecho o inferencia, emitir episodio + opcional CID del `InferenceRecord` JSON.
- **Zyrion:** opcional como **filtro de umbral** sobre Conf de premisas (ya hay `zyrionMinConf`); no sustituye el grafo.
- **Mind:** tras órganos, si hay query de capacidad/self o cambio de hechos → `recomputeKnowledge(organismKey)` → voz + snapshot self.

---

## 10. Self Model

No un struct rígido único: **conjunto de conclusiones** con `Rel` en `{ tiene_capacidad, requiere, disponible, permite_policy, puede_ejecutar }` derivadas por reglas fijas iniciales:

```text
tiene_capacidad(X) ∧ requiere(X,R) ∧ disponible(R)=2 ∧ permite(X)=2 → puede_ejecutar(X)=2
disponible(R)=0 → puede_ejecutar(X)=0
disponible(R)=1 → puede_ejecutar(X)=1
```

---

## 11. Epistemología ≠ autorización

Flujo a respetar (ya parcialmente presente):

```text
observación → hechos → inferencia → (conocimiento)
                                    ↓
                              órganos + ethics
                                    ↓
                              actuate / Core policy
                                    ↓
                              ejecución
```

`puede_ejecutar=2` **no** implica ejecutar; ethics/policy siguen mandando.

---

## 12–13. Persistencia y replicación

- Store local / Supabase / CF según nodo: clave `mind/facts/{organism}` + `mind/inferences/{id}`
- Replicación: mismos blobs que Gen/Mind ya serializan; RootCID puede apuntar a snapshot de hechos+ruleset_version
- Tras migrate node: cargar hechos → `deduceAll` → conclusiones frescas (reconstrucción cognitiva)

---

## 14. Uso por una aplicación real

API mínima (nombres orientativos; alinear a `/api/mind/*` existente):

```text
POST /api/mind/assert   { subject, rel, object, conf, src? }
POST /api/mind/infer    { organism? }  → lista de conclusiones nuevas
POST /api/mind/ask      { subject, rel, object? } → { conf, inference_id, premises, rule }
POST /api/mind/explain  { inference_id }
POST /api/mind/retract  { subject, rel, object }
```

Demo §22: assert capability/resource/policy → ask puede_ejecutar → retract/cambiar conf recurso → ask de nuevo.

---

## 15. Modificación mínima de código

1. Extraer store en memoria (mapa) de `[]ternaryFact` por organismo/sesión.  
2. Reglas fijas `puede_ejecutar` en `deduceAll` o función hermana.  
3. Endpoints assert/ask/infer delgados.  
4. Tests: tres conf de `disponible` → tres conf de `puede_ejecutar`.  
5. Documentar en GUIA.md.  

**Sin** tocar Sales Hub, Latati, ni reescribir tick completo.

---

## 16. Pruebas que demostrarían el valor

| # | Acción | Esperado |
|---|--------|----------|
| 1 | assert tiene_capacidad backup; requiere backup storage; disponible storage 2; permite backup 2; infer; ask puede_ejecutar backup | conf=2 + premises |
| 2 | assert disponible storage 0; infer; ask | conf=0 |
| 3 | retract disponible; assert conf 1; infer; ask | conf=1 |
| 4 | ethics=2 en tick concurrente | no ejecuta aunque conf=2 |

---

## 17. Alternativas A / B / C

| | A Mínima (solo Mind) | **B Híbrida (recomendada)** | C Nativa Core |
|--|----------------------|------------------------------|---------------|
| Dónde vive | solo `mind_reason` | hechos/inferencias en store del nodo; Mind razona | paquete `internal/inference` + API Core |
| Compatibilidad | máxima | alta | media (más superficie) |
| Reuso apps/genes | bajo | medio-alto vía HTTP | alto |
| Complejidad | baja | media-baja | alta |
| Auditabilidad | media | alta si hay InferenceRecord | alta |
| Seguridad | ethics actual | ethics + no auto-exec | policy Core formal |
| Path | prototipo 1 día | **prototipo §22 en días, evolución natural** | después de validar B |

**Recomendación: B.**  
Core no se hincha; Mind sigue siendo el “cerebro”; el store de hechos es reutilizable por Gen/apps sin duplicar motor.

---

## 18. Genes cognitivos

Extensión mínima futura: campo `ruleset` o `manifest.knowledge` en Gen que, al `consult`, inyecte hechos de dominio en el pool de `deduceAll`. **No forzar ahora.**

---

## 19. LLM

Fuera del camino crítico. Opcional: extraer hechos desde lenguaje natural → `assert`. La verdad operativa la fija el motor simbólico.

---

## 20. Casos de uso realistas (3–5)

1. **Self:** puede_ejecutar backup según storage (demo §22).  
2. **Mantenimiento:** temp alta + vibración alta → riesgo (hechos de sensores).  
3. **Comercio (Latati/Abaco):** stock 0 + demanda → no ofertar.  
4. **Seguridad nodo:** peer desconocido + privilegio → ethics sube.  
5. **Clínico ligero (solo trazable):** síntoma + protocolo → “consultar nivel superior” (conf 1 si faltan datos).

---

## 21. Visión y límites de lenguaje

Se demuestra **modelo operacional verificable de sí mismo**, no “consciencia”.  
El salto diferenciador de PrismaTec: hechos + reglas + evidencia + ternario + CID, no más wrappers LLM.

---

## 22. Decisión de arquitectura

| Pregunta | Respuesta corta |
|----------|-----------------|
| ¿Primera clase Inference? | Sí, como registro versionado; no como proceso paralelo a Mind |
| ¿Reescribir Mind? | No |
| ¿Nuevo motor de reglas? | No; extender `deduceAll` |
| ¿Camino? | **B híbrida** |
| ¿Siguiente código? | Solo tras OK explícito: prototipo assert/ask + test §22 |

---

## 23. Diagrama equivalente real (simplificado)

```text
App / Gen / humano
       ↓
   HTTP Mind API
       ↓
  tick / assert / ask
       ↓
  mind_reason (hechos + deduceAll)
       ↕
  episodios CID + store hechos
       ↓
  órganos + ethics + actuate
       ↓
  ejecución (si policy lo permite)
```

Esto ya es casi el diagrama de la visión; falta persistencia de hechos, re-inferencia y self-rules.

---

**Fin del diagnóstico.** Esperando orden explícita para implementar el prototipo mínimo B.
