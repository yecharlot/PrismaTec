# Alset-Gen autónomo + Mind

## Resident explorer

El gen puede vivir fuera de PrismaTec, explorar URLs, guardar hallazgos y **dialogar con Alset Mind**.

### Arranque con anuncio a Mind

```bash
# 1) Exportar paquete desde el nodo
curl -s "https://prismatec.onrender.com/api/gen/export?key=demo-cell" -o demo-cell.package.json

# 2) Daemon (anuncia cada 45s dónde está)
go run ./cmd/alset-gen \
  -package demo-cell.package.json \
  -http :9090 \
  -announce https://prismatec.onrender.com \
  -public-url http://TU_IP_O_NGROK:9090
```

`-public-url` debe ser alcanzable desde el servidor Render (Mind). En local detrás de NAT usa ngrok/cloudflare tunnel.

### API del daemon

| Ruta | Uso |
|------|-----|
| `GET /` | Página de servicio |
| `GET /api/info` | Identidad |
| `POST /api/explore` | Explorar URL (residente) |
| `POST /api/dialogue` | Dialogar / mostrar lo que sabe |
| `GET /api/findings` | Lista de hallazgos |
| `POST /api/pulse` | Pulso genérico |

### Mind localiza y habla

Tras el anuncio, en el nodo:

```bash
curl -s -X POST https://prismatec.onrender.com/api/gen/dialogue \
  -H "Content-Type: application/json" \
  -d '{"key":"demo-cell","text":"quién eres y qué sabes"}' | jq .

# O vía Mind
curl -s -X POST https://prismatec.onrender.com/api/mind/tick \
  -H "Content-Type: application/json" \
  -d '{"text":"habla con gen demo-cell quién eres y qué sabes"}' | jq -r .voice
```

Explore remoto (el gen explora **desde su frontera**, no ida-vuelta vacía):

```bash
curl -s -X POST https://prismatec.onrender.com/api/mind/tick \
  -H "Content-Type: application/json" \
  -d '{"text":"explora https://example.com con gen demo-cell"}' | jq -r .voice
```

### Router / borde

El paquete puede guardarse en un router; el gen **vive** solo si corre `alset-gen` (OpenWrt/Pi/VPS). Mind lo encuentra por el **anuncio** (`http_base`), no por magia DHT.
