# Guía completa de Alset (P.TEC-AN v4.0)

Alset es una red peer-to-peer en Go. Cada proceso del binario es un **nodo**: guarda datos, habla con otros peers, evalúa Lisp, aplica lógica ternaria (Zyrion) y puede servir apps web ancladas a agentes.

Relay público de referencia: **https://prismatec.onrender.com**  
Código: **https://github.com/yecharlot/PrismaTec**  
Landing: **https://yecharlot.github.io/PrismaTec/**

---

## 1. Conceptos

| Concepto | Significado |
|----------|-------------|
| **Nodo local** | Sin variable `RENDER`: API completa, cliente de pulsos, malla libp2p |
| **Nodo relay** | Con `RENDER`: servidor de pulsos y sincronización (sin cliente de pulsos) |
| **Agente** | Identidad en el nodo: `id`, saldo (`balance_utxo`), `root_cid` |
| **CID** | Identificador de contenido (bloque en el blockstore) |
| **DNS Alset** | Alias `nombre.app.ans` → id de agente → contenido del RootCID |
| **LispAI** | Intérprete Lisp embebido vía `POST /api/lispai` |
| **Zyrion** | Unidad lógica con valores 0, 1 y 2 (parcial) |
| **Pulso** | Evento SSE / gossip entre nodos |

---

## 2. Instalación y arranque

### Requisitos

- Go **1.26** o superior
- (Opcional) Supabase: `SUPABASE_URL`, `SUPABASE_SERVICE_KEY`
- (Opcional) Render u otro host con Docker

### Local

```bash
git clone https://github.com/yecharlot/PrismaTec.git
cd PrismaTec
go run ./cmd/prisma-tec
# o: go run ./cmd/prisma-tec 9090
```

- API: `http://localhost:8080`
- Panel: `http://localhost:8080/static/index.html`

### Con Supabase

```bash
export SUPABASE_URL="https://TU_PROYECTO.supabase.co"
export SUPABASE_SERVICE_KEY="tu_secret_key"
go run ./cmd/prisma-tec
```

Sin esas variables, el estado va a `alset_data/` en disco.

### Docker / Render

El `Dockerfile` del repo construye el binario. En Render, define `RENDER` (automático en ese host) y, si aplica, las variables de Supabase.

---

## 3. Modos de operación

| | Local | Relay |
|--|--------|--------|
| Activación | No hay `RENDER` | Existe `RENDER` |
| API HTTP | Sí | Sí |
| Cliente de pulsos | Sí | No |
| Servidor SSE `/api/pulse` | Sí | Sí |
| libp2p | Sí | Sí |

Un solo binario; el modo se elige al arrancar.

---

## 4. API HTTP — referencia

Base local: `http://localhost:8080`  
Base relay: `https://prismatec.onrender.com`

### 4.1 Agentes

| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/api/crear-agente` | Crea agente (saldo inicial 1000) |
| GET | `/api/agentes/` | Lista agentes |
| POST | `/api/eliminar-agente` | Body `{"id":"..."}` |
| POST | `/api/modificar-agente` | Actualiza campos del agente |

```bash
curl -s -X POST https://prismatec.onrender.com/api/crear-agente
curl -s https://prismatec.onrender.com/api/agentes/
```

### 4.2 LispAI

| Método | Ruta | Body |
|--------|------|------|
| POST | `/api/lispai` | `{"cmd":"(expresión lisp)"}` |

Respuesta: `{"resultado": ...}` o `{"error": "..."}`.

```bash
curl -s -X POST https://prismatec.onrender.com/api/lispai \
  -H "Content-Type: application/json" \
  -d '{"cmd":"(+ 1 2 3)"}'
```

Detalle de formas: [LISPAI.md](LISPAI.md).

### 4.3 Contenido (IPFS-like)

| Método | Ruta | Uso |
|--------|------|-----|
| POST | `/api/ipfs/add` | Sube body → CID |
| GET | `/api/ipfs/list` | Lista CIDs |
| GET/POST | `/api/ipfs/fetch` | Obtiene por CID |
| GET | `/api/ipfs/get` | Variante get |
| POST | `/api/ipfs/delete` | Borra bloque |
| POST | `/api/ipfs/clear` | Limpia |

### 4.4 DNS y apps

| Método | Ruta | Uso |
|--------|------|-----|
| GET | `/api/dns/list` | Nombres registrados |
| GET/POST | `/api/dns/resolve` | Resuelve alias → agent_id |
| POST | `/api/dns/delete` | Borra alias |
| POST | `/api/apps/register` | Multipart: `appName` + `files` |
| GET | `/api/apps/list` | Apps en disco |
| GET | `/w/{nombre}.app.ans` | Sirve la app |
| GET | `/apps/...` | Estáticos de apps |

```bash
curl -X POST https://prismatec.onrender.com/api/apps/register \
  -F "appName=demo" \
  -F "files=@index.html"
# Abrir: https://prismatec.onrender.com/w/demo.app.ans
```

### 4.5 Red y sincronización

| Método | Ruta |
|--------|------|
| GET | `/api/network/peers` |
| GET | `/api/sync/status` |
| POST | `/api/sync/full` |
| POST | `/api/sync/quick` |
| GET/POST | `/api/sync/config` |
| GET | `/api/pulse` (SSE) |
| POST | `/api/pulse/emit` |

### 4.6 IA neural (HTTP)

| Método | Ruta |
|--------|------|
| POST | `/api/ia/configurar` |
| POST | `/api/ia/inferir` |
| GET | `/api/ia/estado` |
| * | `/api/ia/sinapsis`, `/api/ia/memoria` |
| * | `/api/ia/topologia`, `/api/ia/metricas`, conectar/clear sinapsis |

### 4.7 Auth, roles, módulos, entidades

Ver [API_AUTH_MODULES_V2.md](API_AUTH_MODULES_V2.md).

Resumen:

- `POST /api/auth/token` · `validate` · `revoke`
- `POST /api/roles` · `GET /api/roles/{id}`
- `GET/POST /api/modulos` · `GET/PUT/DELETE /api/modulos/{id}`
- `GET/POST /api/entidades` · `GET/POST /api/relaciones`

### 4.8 API v2

| Método | Ruta | Uso |
|--------|------|-----|
| GET | `/api/v2/info` | Info del nodo |
| POST | `/api/v2/agente/crear` | `{"id":"…","balance":1000}` |
| POST | `/api/v2/transferir` | `{"from","to","amount","token?"}` |
| POST | `/api/v2/app/publicar` | JSON o body raw |
| POST | `/api/v2/app/instalar` | `{"cid","name"}` |
| POST | `/api/v2/app/ejecutar` | Devuelve HTML |

### 4.9 Prism, PoH, admin, debug

| Ruta | Uso |
|------|-----|
| `/api/prism/sellar` · `verificar` · `revocar` | Sellado / verificación |
| `/api/poh/event` · `/api/poh/proof` | Proof of Humanity (experimental) |
| `/api/admin/login` · `update-pass` | Admin panel |
| `/api/audit/log` · `/api/debug/estado` | Auditoría y estado |

---

## 5. LispAI — qué puedes hacer

Documentación extendida: [LISPAI.md](LISPAI.md).

### Núcleo

`quote`, `if`, `progn`, `let`, `lambda`, `defun`, `setq`, aritmética, listas (`car`/`cdr`/`cons`/`mapcar`/`elemento`).

### Agentes y red

```lisp
(crear-agente "alice")
(get-agent-balance "alice")
(set-agent-balance "alice" 50)
(register-name "alice.app.ans" "alice")
(ipfs-add "hola")
(host-id)
(net-peers)
```

### Zyrion

```lisp
(zyrion (list 1 0 1))                    ; → 2
(zyrion-network topo entradas)
```

### DSL de decisión

```lisp
(evaluar-zyrion
  (quote (N :entradas (X Y) :salidas ((0 BLOQUEO) (1 OK) (2 RARO))))
  (quote (X 0.1 Y 0.1)))
; → BLOQUEO
```

Umbrales continuo→ternario: `<0.33→0`, `<0.66→1`, resto `→2`.  
Subnodos anidados e `INVOCAR_RED_NEURONAL` soportados.

### Modelos ligeros

```lisp
(embedding "texto")
(ternarizar …) (desternarizar …) (similitud a b)
(crear-capa-lineal "c1" 32 8)
(crear-modelo "m1" (list "c1"))
(inferir "m1" (ternarizar (embedding "hola")))
(entrenar-hebbiano "m1" dataset epocas tasa)
```

---

## 6. Flujos típicos

### Publicar una app

1. `POST /api/apps/register` con archivos, o Lisp + `set-agent-root` + `register-name`
2. Abrir `/w/nombre.app.ans`

### Contrato lógico

1. Definir topología con `evaluar-zyrion`
2. Probar escenarios (valores 0–1 en el entorno)
3. Opcional: anclar el texto del acuerdo con `ipfs-add` y el RootCID del agente

### Automatización

Un `progn` en `/api/lispai`: crear agente → registrar DNS → subir payload → emitir lógica Zyrion.

### Transferencia de saldo de aplicación

```bash
curl -s -X POST …/api/v2/transferir \
  -H "Content-Type: application/json" \
  -d '{"from":"alice","to":"bob","amount":10}'
```

No es liquidación blockchain; es saldo **dentro de la red Alset**.

---

## 7. Persistencia

| Destino | Cuándo |
|---------|--------|
| `alset_data/` | Por defecto |
| Supabase | Si hay `SUPABASE_URL` + `SUPABASE_SERVICE_KEY` |
| CIDs en disco | Bloques bajo el directorio de blocks del nodo |

`defun`/`setq` en Lisp viven en **RAM** hasta reinicio. Lo importante debe ir a agente, CID o store.

---

## 8. Tests

```bash
go test ./... -count=1
go test ./internal/lisp/ -v
```

---

## 9. Estructura del repositorio

```
cmd/prisma-tec/          Entrada
internal/node/           Nodo, HTTP, P2P, sync, API v2
internal/lisp/           Motor Lisp, power, evaluar-zyrion
internal/httpapi/        Montaje de rutas y Backend
internal/persistence/    Disco + Supabase
internal/agents/         Tipos agente, módulo, token
internal/neural/         Estado neural
internal/pulse/          SSE
internal/poh/            Proof of Humanity
docs/                    Esta guía y anexos
static/                  Panel y apps
```

Arquitectura detallada: [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 10. Límites honestos

- No sustituye una blockchain pública ni custodia legal de valor.
- Zyrion/modelos son experimentales, no un LLM de producción.
- Tokens y módulos están en memoria del proceso (se pierden al reiniciar salvo que se reemitan).
- Un relay solo muestra `peers: 0` hasta que otros nodos se conecten.

---

## 11. Enlaces rápidos

| Recurso | URL |
|---------|-----|
| Repo | https://github.com/yecharlot/PrismaTec |
| Relay | https://prismatec.onrender.com |
| Landing | https://yecharlot.github.io/PrismaTec/ |
| LispAI | [LISPAI.md](LISPAI.md) |
| Auth / v2 | [API_AUTH_MODULES_V2.md](API_AUTH_MODULES_V2.md) |
| Arquitectura | [ARCHITECTURE.md](ARCHITECTURE.md) |
