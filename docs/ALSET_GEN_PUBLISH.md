# Publicar y resolver genes (CID + DNS TXT)

## Publicar

```bash
curl -s -X POST https://prismatec.onrender.com/api/gen/publish \
  -H "Content-Type: application/json" \
  -d '{"key":"demo-cell"}' | jq .
```

Respuesta: `package_cid` + `urls` (by-cid del nodo y, si existe, `IPFS_GATEWAY`).

Recuperar el JSON:

```bash
curl -s "https://prismatec.onrender.com/api/gen/by-cid?cid=bafk..." -o demo-cell.package.json
```

Daemon desde URL (sin archivo local):

```bash
go run ./cmd/alset-gen \
  -package-url "https://prismatec.onrender.com/api/gen/by-cid?cid=bafk..." \
  -http :9090 -udp 9091 \
  -announce https://prismatec.onrender.com \
  -public-url https://TU-TUNEL
```

## Variables de entorno (nodo)

| Variable | Uso |
|----------|-----|
| `ALSET_PUBLIC_BASE` | URL pública del nodo (urls de publish) |
| `RENDER_EXTERNAL_URL` | Fallback automático en Render |
| `IPFS_GATEWAY` | Gateways extra, separados por coma (`https://ipfs.io/ipfs`) |
| `ALSET_DNS_SUFFIX` | Sufijo DNS para TXT (`ejemplo.com` → `demo-cell.ejemplo.com`) |

## DNS TXT (soberano, no root DNS)

En **tu** zona DNS:

```text
demo-cell.tudominio.com.  TXT  "alset-pkg=bafk... alset-reach=https://tu-borde.ejemplo"
```

o:

```text
_alset.demo-cell.tudominio.com.  TXT  "alset-pkg=bafk..."
```

```bash
curl -s "https://prismatec.onrender.com/api/gen/resolve?key=demo-cell" | jq .
```

Mind: `resuelve gen demo-cell`

## Revive global

```bash
curl -s -X POST https://prismatec.onrender.com/api/gen/revive \
  -d '{"package_cid":"bafk..."}' | jq .
```

Si el CID no está en el disco local, el nodo intenta **by-cid público / gateways**.
