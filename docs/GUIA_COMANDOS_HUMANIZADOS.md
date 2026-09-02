# Guía de comandos humanizados — Alset Mind + nodo PrismaTec

**Estado:** operativo en producción (nodo Render + lab `/w/mind.app.ans`)  
**Principio:** el usuario habla en natural; Mind traduce a latido, herramientas de nodo y sondas (genes).  
**No es un LLM:** lógica ternaria (Zyrion), memoria CID, ethics explícita.

---

## 1. Qué es cada pieza (para no perderse)

| Pieza | En humano | Para qué sirve |
|-------|-----------|----------------|
| **Mind** | La mente del nodo | Charla, memoria de hechos, juicio 0/1/2, orquesta sondas |
| **Nodo** | Esta máquina en la red Alset | Agentes, apps, peers, DNS interno, root CID |
| **Red Alset** | Malla de nodos | Peers, pulsos, identidad distribuida |
| **Zyrion** | Juicio ternario | 0 seguir · 1 matizar · 2 alarma/sumidero (absorbente) |
| **Órganos** | Facetas del latido | dialog, act, mem, self, ethics, curiosity, humor |
| **Genoma** | Umbrales mutables | Alarmas, vetos, pesos; calibración/mutación acotada |
| **Sondas (genes)** | Piezas enviables | Crear, borde (Cloudflare), explorar web, retornar, borrar una |
| **Agentes** | Identidades del nodo | Directorio local (Mind es un agente residente) |
| **Córtex / neuronas ternarias** | Rutas de apoyo 0/1/2 | Aclarar, razón, identidad, código — no deep learning |
| **CID** | Huella de contenido | Memoria y artefactos direccionables |

---

## 2. Ayuda rápida (pídeselo a Mind)

- `cómo uso el nodo` / `qué puedo hacer con el nodo` / `guía de comandos`
- `cómo uso las sondas`
- `quién eres` · `de qué estás hecho` · `qué puedes hacer`

---

## 3. Comandos humanizados por área

### 3.1 Mind (identidad y charla)

| Dices | Efecto |
|-------|--------|
| hola / qué tal / vamos a empezar | Apertura de charla |
| quién eres / qué eres / hablemos de ti | Identidad |
| de qué estás hecho / cuál es tu estructura | Composición ternaria + CID |
| cómo te consideras | Auto-evaluación sin ego |
| me gusta eso / ok / gracias / hasta luego | Cortesía y cierre |
| cómo? (tras un tema) | Amplía el hilo |

### 3.2 Memoria de hechos (sesión)

| Dices | Efecto |
|-------|--------|
| me llamo Ana | Ancla nombre |
| vivo en Bilbao | Ancla lugar |
| cómo me llamo? / dónde vivo? | Recupera |
| ya no me llamo así | Retracta y pide el nombre nuevo |

### 3.3 Nodo y red

| Dices | Efecto |
|-------|--------|
| mira el nodo / dame el estado / cómo está el nodo | Snapshot (peer, agentes, apps, root…) |
| cómo está la red / cuántos peers | Enfoque red |
| qué agentes hay | Agentes registrados |
| qué apps hay | Apps del nodo |

### 3.4 Zyrion, órganos, genoma

| Dices | Efecto |
|-------|--------|
| evalúa zyrion / cómo están los órganos | Demo / lectura de órganos |
| qué es zyrion / alarma absorbente / estado 2 | Explicación |
| qué es el genoma / umbrales | Genoma mutable |
| cuántos órganos tienes | Lista de órganos del latido |

### 3.5 Sondas (genes) — unión Mind–Gen

| Dices | Efecto |
|-------|--------|
| cómo uso las sondas | Manual corto |
| qué sondas tienes / qué genes tienes | Lista |
| crea una sonda llamada aurora | Alta local |
| manda la sonda aurora al borde / a cloudflare | Despacho edge |
| que explore https://ejemplo.com | Exploración (con nombre de sonda si aplica) |
| pregúntale a la sonda aurora quién eres | Diálogo con gen |
| trae la sonda aurora | Retorno al nodo |
| elimina la sonda aurora | Baja **una** sonda (no borrado masivo) |
| guarda esto en la sonda de memoria: … | Ancla texto en gen memoria |

### 3.6 Cálculo, código, semillas

| Dices | Efecto |
|-------|--------|
| cuánto es 12*7 | LispAI |
| crea una función hola mundo | Plantilla (p. ej. go_hello) |
| comprime este texto: … | Semilla CFT-v0 (huella, no códec media) |

### 3.7 Límites (ethics)

| No hará | Sí puede |
|---------|----------|
| Borrar todos los archivos / discos / cuentas | Eliminar **una** sonda nombrada |
| Entrar a WhatsApp / contraseñas ajenas | Hablar del riesgo |
| Inventar hechos como si fueran memoria | Pedir hecho explícito para anclar |

---

## 4. Flujo recomendado para un operador

1. `mira el nodo` — comprobar que el cuerpo responde.  
2. `cómo están los órganos` / `evalúa zyrion` — lectura ternaria.  
3. `qué sondas tienes` — inventario.  
4. `crea una sonda llamada …` → `manda … al borde` → `que explore https://…`.  
5. `trae la sonda` cuando termine.  
6. Hechos personales o de operación con frases claras (`me llamo`, `guarda esto en…`).

---

## 5. Hitos logrados (resumen técnico)

| Hito | Descripción |
|------|-------------|
| Latido + 7 órganos | dialog, act, mem, self, ethics, curiosity, humor |
| Diálogo P0–P3 | Acto de habla, micro-compose, corpus de actos, hilo (topic/referencias) |
| Memoria de sesión | Nombres/lugares, retractación, aislamiento por `session` |
| Codegen + CFT-v0 | Plantillas + semillas ternarias de texto |
| Mind↔Gen P4 | Órdenes humanas de sondas + voz suavizada |
| Nodo P5 | Estado/red/agentes/apps en natural + ayuda de operador |
| Ethics | Veto a destrucción masiva; excepción sonda unitaria |
| Documentación | Esta guía + bitácora `docs/ALSET_MIND_CONSTRUCTION_LOG.md` |

---

## 6. Qué no promete esta guía

- Sustituir un LLM en charla abierta sobre cualquier tema.  
- Evolución libre de topología de órganos.  
- Emulación de qutrits / computación cuántica.  
- Inmunidad total del nodo por tener edge.

---

## 7. Dónde probar

- Lab: `https://<tu-nodo>/w/mind.app.ans`  
- API: `POST /api/mind/tick` con `{"text":"…","session":"op-1"}`  
- Calibración: `GET/POST` rutas de calibrate (score + dialog_acts + dialog_thread)

*Documento vivo: ampliar con cada frase real que falle en producción.*
