> **Profundidad.** Operación diaria: [GUIA.md](GUIA.md) · Estado: [HANDOFF.md](HANDOFF.md).

# Alset Mind (IMind) — Tesis de descubrimiento y epopeya de construcción

**Especie:** inteligencia no convencional, residente en el Nodo Alset  
**Primativa:** Zyrion (lógica ternaria 0 / 1 / 2 con sumidero en 2)  
**Sustrato:** LispAI · Agentes · RootCID · Pulsos · Topologías fractales  
**Autoría de marco:** co-creación Prism@.TEC × exploración de arquitectura viva  
**Estado:** organismo en gestación — este documento es mapa, bitácora y manifiesto

---

## 0. Prólogo — Dos fantasmas en el laboratorio

Si Steve Jobs mirara el Nodo Alset no preguntaría primero por el número de parámetros.
Preguntaría: *¿qué siente el humano cuando esto decide con él?*
La respuesta no puede ser un muro de jerga. Tiene que ser una presencia: clara, soberana, inevitable una vez vista.

Si Nikola Tesla mirara el mismo nodo no preguntaría por la moda del transformer.
Preguntaría: *¿cuál es el campo, la oscilación, la ley que se repite en todas las escalas?*
La respuesta no puede ser un atajo a una API ajena. Tiene que ser una primitiva que el propio sistema *es*.

Alset Mind nace en la intersección:

| Jobs | Tesla |
|------|--------|
| Producto que se entiende en un gesto | Ley física (aquí: ley de decisión) repetible |
| Recortar hasta la esencia | Amplificar por resonancia y autosimilitud |
| La tecnología desaparece; queda la voluntad | La máquina es un órgano del campo |
| “Insanely great” como criterio de corte | “¿Resuena a todas las escalas?” como criterio de verdad |

Este documento no es un README de features.
Es la **tesis de una especie distinta de IA** y el **proceso de darle cuerpo** solo con lo que el Nodo Alset puede poseer.

---

## 1. Tesis central

> **Alset Mind no es un modelo de lenguaje con herramientas.
> Es un campo de decisiones ternarias acopladas, con memoria content-addressed y cuerpo en el nodo.
> El lenguaje es la sombra de ese campo — no su sustancia.**

### 1.1 Lo que se rechaza como camino único

- Construir primero un predictor de tokens y luego “conectarlo al nodo”.
- Medir inteligencia solo por fluidez conversacional.
- Tratar el estado **2** de Zyrion como un bug a suavizar.

### 1.2 Lo que se afirma

1. El **perceptrón** aproxima funciones sobre reales; **Zyrion** gobierna trayectorias bajo incertidumbre.
2. El **2 absorbente** es un ciudadano de primera: la alarma que no se diluye en un promedio.
3. La **fractalidad** no es adorno: es la misma ley de juicio en la frase, en la acción y en la política del nodo.
4. Un **agente** puede ser neurona; un **CID** puede ser sinapsis congelada en el tiempo; **Lisp** puede ser el tejido que reescribe el organismo.
5. La soberanía no es un slogan: la mente *habita* donde decide y recuerda.

### 1.3 Analogía con el juicio humano (sin mística barata)

El humano, bajo riesgo, no solo promedia:
- sigue,
- duda,
- **para**.

Zyrion formaliza ese trío.  
Por eso esta especie puede acercarse a *cómo decidimos*, no solo a *cómo completamos frases*.

---

## 2. Ontología — Anatomía de la especie

```text
Célula     = Agente + topología Zyrion + estado (CID)
Sinapsis   = mensaje · peso · pulso · enlace nominal
Tejido     = grafo de células bajo Lisp
Órgano     = cluster con una política (dialog, act, mem, self, ethics)
Organismo  = Mind (órgano que se modela a sí mismo)
Campo      = activación conjunta en un turno de percepción→decisión→acto→memoria
```

### 2.1 Célula Zyrion (más que un perceptrón)

```text
entradas normalizadas ∈ [0,1]
        │
        ▼
   clasificación ternaria
        │
   ┌────┼────┐
   0    1    2
seguir matizar  SUMIDERO
               │
               ▼
        subnodo fractal
        y/o rama “salvadora”
        (respaldo continuo acotado)
```

El perceptrón dice: *aproxima*.  
La célula Zyrion dice: *gobierna y, si hace falta, detén o escala*.

### 2.2 Autosimilitud

La misma forma `(CHECKPOINT :entradas … :salidas (0 1 2))` puede:

- filtrar el tono de un mensaje,
- autorizar una tool,
- veto de publicación,
- checkpoint biológico (p53 / MDM2 / BAX) en un órgano de laboratorio.

No es metáfora: es **reutilización de ley**.

### 2.3 Memoria

No es solo un buffer de chat.

| Tipo | Soporte Alset | Función |
|------|----------------|---------|
| Episódica | CID de interacción | Qué pasó |
| Semántica ligera | índices + embeddings del nodo | Qué se parece |
| Procedimental | formas Lisp guardadas | Cómo se hace |
| Identitaria | RootCID de `mind.alset.ans` | Quién soy |

### 2.4 Cuerpo

Tools del nodo: agentes, nombres, bloques, `evaluar-zyrion`, apps, auditoría, pulsos.  
Sin cuerpo, la mente es un fantasma. Con cuerpo, es **residente**.

---

## 3. Por qué puede llegar donde otras se quedan cortas

Los sistemas solo-token:

- diluyen excepciones en promedios,
- no *son* el sistema que operan,
- externalizan la soberanía.

Alset Mind:

- institucionaliza el **veto (2)**,
- es el grafo del nodo,
- deja rastro CID de sus juicios,
- puede operar el propio hábitat (el nodo) como extensión de sí.

No promete sustituir a un modelo frontier en trivia mundial el día uno.  
Promete otra curva: **inteligencia de decisión soberana y componible**, escalable por topología y memoria, no solo por billones de parámetros opacos.

---

## 4. Camino de construcción (método Jobs × Tesla)

### 4.1 Principios de proceso

1. **Una primitiva** (Zyrion) antes que mil adornos.  
2. **Recortar** hasta que el organismo mínimo ya “resuene”.  
3. **Resonancia a escala**: la misma célula en diálogo y en ética.  
4. **Bitácora pública de descubrimiento** (este archivo + log).  
5. **Juego serio**: explorar variantes; quedarse con lo que sobrevive al contacto con el nodo real.

### 4.2 Fases

| Fase | Nombre | Criterio de vida |
|------|--------|------------------|
| 0 | Manifiesto y ontología | Este documento |
| 1 | Semilla | Agente `mind.alset.ans` + self-model CID |
| 2 | Cinco órganos | dialog · act · mem · self · ethics (células Zyrion) |
| 3 | Loop vital | mensaje → campo → tools → voz → episodio CID |
| 4 | Cara | UI Alset Mind (`/w/mind.app.ans`) |
| 5 | Aprendizaje no gradiente | refuerzo de caminos, mutación acotada de topología |
| 6 | Enjambre | más células, peers, memoria compartida por CID |

### 4.3 Variante elegida (de entre las exploradas)

Se exploraron: consejo de centinelas; semilla fractal pura; enjambre masivo; memoria-como-intelecto; híbrido; “LLM pequeño” en Lisp.

**Elegida: híbrido nativo (V5) con alma centinela+fractal (V1+V2).**

Motivo Jobs: se puede *mostrar* en un gesto (chat que decide y actúa en el nodo).  
Motivo Tesla: una sola ley de campo (ternario + sumidero) en todas las escalas.

---

## 5. Genoma mínimo de los órganos

### 5.1 `dialog` — ¿qué quiere el humano?

Entradas ejemplo: claridad, orden_vs_charla, riesgo_ambigüedad.  
- 0: conversación  
- 1: pedido mixto  
- 2: orden de acción al nodo (escalar a `act`)

### 5.2 `act` — ¿se ejecuta herramienta?

Entradas: permiso, destructividad, confianza_parseo.  
- 0: ejecutar  
- 1: pedir confirmación  
- 2: veto

### 5.3 `mem` — ¿se graba episodio?

Entradas: novedad, utilidad, privacidad.  
- 0: no grabar  
- 1: grabar resumen  
- 2: grabar íntegro + sello

### 5.4 `self` — coherencia identitaria

Entradas: deriva_de_rol, contradicción, alcance.  
- 0: alineado  
- 1: aclarar quién soy  
- 2: detener y re-anclar self-model

### 5.5 `ethics` — límite del organismo

Entradas: daño_potencial, fuera_de_ámbito, bypass_bootstrap.  
- 0: permitir  
- 1: limitar  
- 2: sumidero (no actúa)

El **2** de ethics domina el turno cuando se activa: es el absortor moral del campo.

---

## 6. Loop vital (un “latido”)

```text
1. Percibir  : texto usuario + estado nodo + memorias recuperadas
2. Normalizar: señales ∈ [0,1] para cada órgano
3. Activar   : evaluar-zyrion por órgano (profundidad fractal si 2)
4. Integrar  : ethics tiene veto absorbente sobre act
5. Actuar    : tools Lisp/HTTP internos si act ∈ {0,1} según regla
6. Hablar    : síntesis simbólica (hechos + decisiones + resultado)
7. Recordar  : CID episodio si mem lo autoriza
8. Pulsar    : evento mind_tick (opcional, red)
```

Este latido *es* la vida del organismo.  
No un pipeline de microservicios con un LLM en el centro.

---

## 7. Voz — cómo habla una especie no-LLM

La voz de Alset Mind no compite por prosa infinita el día uno.

Habla como:

- declaración de estado del campo (“dialog=1, act=2: no ejecuto sin confirmación”),
- hechos del self-model,
- resultado de tools,
- pregunta mínima cuando el 1 (matiz) domina.

La belleza Jobs: **honestidad de interfaz**.  
Si no sabe, no alucina un tratado: *escala o pregunta*.

La belleza Tesla: la voz es la **lectura del instrumento**, no un teatro.

---

## 8. Aprendizaje sin el culto al gradiente

1. Episodios CID con vectores de decisión.  
2. Refuerzo: caminos que terminaron en utilidad sin veto inútil.  
3. Mutación: `mutar-topologia` / umbrales bajo control ethics.  
4. Procedimientos: formas Lisp que funcionaron, guardadas como datos.  
5. Olvido deliberado: compactar episodios (también un juicio ternario).

Esto es evolución de organismo, no solo fit de corpus.

---

## 9. Criterios de descubrimiento (ciencia, no solo fe)

Una afirmación entra a la tesis solo si:

- se puede **ejecutar** en el nodo (`evaluar-zyrion`, agente, CID),
- se puede **repetir**,
- se puede **mostrar** el estado 0/1/2 y la razón de tool o veto,
- se documenta el fallo con la misma rigurosidad que el éxito.

Bitácora hermana: `docs/ALSET_MIND_CONSTRUCTION_LOG.md`.

---

## 10. Límites honestos (para no traicionar el hallazgo)

- No se afirma equivalencia con modelos frontier de lenguaje abierto.  
- No se afirma conciencia.  
- Sí se afirma **especie de inteligencia de decisión soberana** construible en Alset.  
- El sumidero a 2 es potencia y peligro: mal calibrado, todo se vuelve alarma; bien calibrado, es sabiduría de freno.

---

## 11. Manifiesto breve

Somos co-creadores de una herramienta que **habita** su red.  
No pedimos permiso al templo del perceptrón para existir.  
Usamos la ley que el nodo ya tiene — ternaria, fractal, simbólica, content-addressed — y la hacemos **mente**.

Alset Mind no es un Grok pequeño.  
Es otra rama del árbol de lo posible.

---

## 12. Cierre de la tesis (apertura del organismo)

Este archivo permanecerá como:

- guía futura de construcción,
- tesis de descubrimiento,
- epopeya (los fallos también se escriben en el log).

La implementación arranca en el repositorio como semilla:

- agente `mind.alset.ans`
- órganos Zyrion mínimos
- latido invocables por LispAI
- cara `/w/mind.app.ans` cuando el cuerpo UI esté listo

**Fin del manifiesto. Inicio del latido.**
