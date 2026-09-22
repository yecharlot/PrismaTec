# Auth, módulos y API v2

Estas piezas ya operan en el nodo (salvas 2026 + capa v2).

## Auth tokens

| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/api/auth/token` | Emite token ligado a un `agent_id` y roles |
| GET/POST | `/api/auth/validate` | Valida un token |
| POST | `/api/auth/revoke` | Revoca un token |
| POST | `/api/roles` | Asigna roles a un agente |
| GET | `/api/roles/{agent_id}` | Lista roles |

Ejemplo:

```bash
curl -s -X POST http://localhost:8080/api/auth/token \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"alice","roles":["editor"],"horas":24}'
```

Roles típicos: `admin` (permiso `*`), `editor`, etc. Los tokens viven en memoria del proceso.

## Módulos y entidades

| Método | Ruta |
|--------|------|
| GET/POST | `/api/modulos` |
| GET/PUT/DELETE | `/api/modulos/{id}` |
| GET/POST | `/api/entidades` |
| GET | `/api/entidades/{id}` |
| GET/POST | `/api/relaciones` |

```bash
curl -s -X POST http://localhost:8080/api/modulos \
  -H "Content-Type: application/json" \
  -d '{"nombre":"pagos","rol":"core","owner":"alice"}'
```

## API v2 (desarrolladores)

| Método | Ruta | Uso |
|--------|------|-----|
| GET | `/api/v2/info` | Metadatos del nodo y endpoints |
| POST | `/api/v2/agente/crear` | Crear agente (`id` opcional, `balance` default 1000) |
| POST | `/api/v2/transferir` | Transferir saldo (`from`, `to`, `amount`, `token` opcional) |
| POST | `/api/v2/app/publicar` | Publicar contenido como app (`name`+`content` JSON o body raw) |
| POST | `/api/v2/app/instalar` | Instalar desde CID (`cid`, `name`) |
| POST | `/api/v2/app/ejecutar` | Devuelve HTML del CID o nombre `.app.ans` |

```bash
curl -s http://localhost:8080/api/v2/info
curl -s -X POST http://localhost:8080/api/v2/agente/crear \
  -H "Content-Type: application/json" \
  -d '{"id":"bob","balance":500}'
curl -s -X POST http://localhost:8080/api/v2/transferir \
  -H "Content-Type: application/json" \
  -d '{"from":"alice","to":"bob","amount":10}'
```

