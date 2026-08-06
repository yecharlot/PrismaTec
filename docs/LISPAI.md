# LispAI — motor embebido del nodo

Cada nodo Alset incluye un intérprete Lisp invocable por HTTP. Sirve para automatizar agentes, contenido, reglas ternarias (Zyrion) y modelos ligeros guardados como agentes + CID.

## Endpoint

```http
POST /api/lispai
Content-Type: application/json

{"cmd": "(+ 1 2 3)"}
```

Respuesta correcta: `{"resultado": ...}`  
Error de evaluación: `{"error": "..."}`  
Algunos builtins devuelven el error como string dentro de `resultado`.

| Entorno | URL base |
|---------|----------|
| Local | `http://localhost:8080` |
| Relay | `https://prismatec.onrender.com` |

```bash
curl -s -X POST http://localhost:8080/api/lispai \
  -H "Content-Type: application/json" \
  -d '{"cmd":"(+ 1 2 3)"}'
```

El estado de `setq` / `defun` vive en la **RAM del proceso** hasta reinicio. Lo que debe sobrevivir se guarda en agentes, CIDs o Supabase.

---

## Núcleo del lenguaje

Formas especiales: `quote`, `if`, `progn`, `let`, `let*`, `lambda`, `defun`, `defmacro`, `setq`, `defvar`.

Aritmética y listas: `+ - * /`, comparaciones, `list`, `car`/`cdr`/`cons`, `mapcar`, `append`, `length`, `elemento`, `drop`, `range`.

---

## Agentes, red y contenido

| Forma | Descripción |
|-------|-------------|
| `(crear-agente "id")` | Crea agente (saldo inicial **1000**) |
| `(get-agent-balance "id")` / `(get-balance "id")` | Lee UTXO |
| `(set-agent-balance "id" 50)` / `(set-balance …)` | Escribe UTXO y sincroniza |
| `(get-agent-root "id")` | CID ancla del agente |
| `(set-agent-root "id" "bafk…")` | Asigna RootCID |
| `(register-name "alias.app.ans" "id")` | DNS interno → `/w/alias.app.ans` |
| `(ipfs-add "texto")` | CID + anuncio en la malla |
| `(fetch-cid "bafk…")` | Lee bloque |
| `(host-id)` | Peer ID del nodo |
| `(net-peers)` | Número de peers conectados |

Las **apps web** también se registran por HTTP:

```bash
curl -X POST https://prismatec.onrender.com/api/apps/register \
  -F "appName=config" \
  -F "files=@static/apps/config/index.html"
```

Respuesta incluye `url` del estilo `/w/config.app.ans`.

---

## Zyrion (lógica ternaria)

Valores: **0** (falso), **1** (verdadero), **2** (parcial / indeterminado).

```lisp
(zyrion (list 1 1 1))           ; → 1
(zyrion (list 1 0 1))           ; → 2

;; Topología: índices negativos = entradas externas (-1 = primera)
(zyrion-network (list (list -1 -2)) (list 1 0))
```

También: `zyrion-network-parallel`, `topologia-aleatoria`, `mutar-topologia`, `cruzar-topologias`, `expandir-fractal`, `fitness-topologia`, `evolucionar-xor`.

---

## Embeddings y modelos ternarios

Recuperados de las salvas del nodo (2026). No son un LLM de producción: son **vectores y grafos ternarios** almacenados como agentes.

| Forma | Descripción |
|-------|-------------|
| `(embedding "texto")` | SHA-256 → lista de 32 floats 0–1 |
| `(ternarizar lista-o-num)` | Continuo → {0,1,2} |
| `(desternarizar …)` | {0,1,2} → {0, 0.5, 1} |
| `(similitud a b)` | Textos o listas; 1.0 = idénticos en ternario |
| `(crear-capa-lineal "capa" in out)` | Agente + topología aleatoria en RootCID |
| `(crear-modelo "mod" (list "capa1" "capa2"))` | Encadena capas |
| `(inferir "mod" entrada-lista)` | Evalúa con `zyrion-network` por capa |
| `(entrenar-hebbiano "mod" dataset epocas tasa)` | Ajuste exploratorio (máx. 500 épocas) |

Ejemplo mínimo:

```lisp
(progn
  (crear-capa-lineal "c1" 32 8)
  (crear-modelo "m1" (list "c1"))
  (inferir "m1" (ternarizar (embedding "hola"))))
```

Dataset de autoencoder:

```lisp
(list
  (list (embedding "hola") (ternarizar (embedding "hola")))
  (list (embedding "red") (ternarizar (embedding "red"))))
```

---

## Utilidades

`(random)`, `(sha256 "…")`, `(to-json …)`, `(from-json "…")`, `(current-time)`, `(println …)`, `(log …)`.

---

## Límites conscientes

- Una expresión (o un `progn`) por petición HTTP.
- `evaluar-zyrion` con DSL nombrado (`:entradas` / `:salidas`) **aún no** está reintroducido; usar `zyrion-network` por índices.
- El entrenamiento Hebbiano es exploración de topología, no backprop.
- Reiniciar el proceso borra `defun`/`setq` en RAM; persiste lo anclado a agentes/CID.


## evaluar-zyrion (DSL de decisión)

Topologías legibles con `:entradas` y `:salidas`. Los valores del entorno en continuo se mapean a ternario: `<0.33→0`, `<0.66→1`, resto `→2`.

```lisp
(evaluar-zyrion
  (quote (CONTRATO
    :entradas (PagoRecibido EntregablesVerificados)
    :salidas (
      (0 ANULACION)
      (1 (DISPUTA :entradas (MargenTiempo)
           :salidas ((0 PENALIZACION) (1 STANDBY) (2 INVOCAR_RED_NEURONAL))))
      (2 LIQUIDAR))))
  (quote (PagoRecibido 1.0 EntregablesVerificados 0.3 MargenTiempo 0.9
          Valor_Continuous_Neural 0.75)))
```

- La salida del nodo es el resultado de `(zyrion entradas-ternarias)`.
- Una salida puede ser una **etiqueta** o un **subnodo** con la misma forma.
- `INVOCAR_RED_NEURONAL` usa `Valor_Continuous_Neural` del entorno o `local-inference`.

