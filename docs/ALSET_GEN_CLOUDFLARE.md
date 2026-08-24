> **Preferir** [`ALSET_NETWORK_LIVE.md`](ALSET_NETWORK_LIVE.md) + [`ALSET_CF_STORE.md`](ALSET_CF_STORE.md).

# Alset-Gen en Cloudflare (torrente de borde legítimo)

## Por qué sí (y en qué se diferencia del DNS raíz)

| | DNS raíz / SCION CORE | Cloudflare Workers + DO |
|--|----------------------|-------------------------|
| ¿Ejecutas tu código? | No | **Sí**, en el edge |
| ¿Violación de infra ajena? | Intentarlo sería abuso | **Contrato de uso** de CF: es su producto |
| ¿Disponibilidad global? | Para *resolver* / *enrutar* | Para **tu Worker** en ~300+ ciudades |
| ¿Encaja Alset? | Solo como metáfora | **Runtime real** de célula en el camino |

Cloudflare **no** es descentralización soberana pura (es un proveedor).  
**Sí** es el torrente práctico: borde global, HTTP, estado (Durable Objects), sin montar tu propio anycast.

## Despliegue

```bash
cd cloudflare/alset-gen-worker
npm install
npx wrangler login
npx wrangler secret put ANNOUNCE_URL   # https://prismatec.onrender.com
npx wrangler secret put PUBLIC_URL    # https://alset-gen.<cuenta>.workers.dev
# opcional: ROOT_CID / PACKAGE_CID en [vars] tras publish
npx wrangler deploy
```

Anunciar a Mind:

```bash
curl -s -X POST https://alset-gen.<cuenta>.workers.dev/api/announce-now
```

Dialogar:

```bash
curl -s -X POST https://prismatec.onrender.com/api/gen/dialogue \
  -H "Content-Type: application/json" \
  -d '{"key":"demo-cell","text":"quién eres y qué sabes"}' | jq .
```

## API (compatible con el daemon Go)

| Ruta | Función |
|------|---------|
| `GET /` | Página de la célula |
| `GET /api/info` | Identidad |
| `POST /api/dialogue` | Voz / hallazgos |
| `POST /api/explore` | Explore no invasivo desde el edge |
| `GET /api/findings` | Lista (DO) |
| `POST /api/announce-now` | Registro en PrismaTec |

## Stack completo Alset-Gen

```text
package_cid (IPFS / by-cid)     → no muere
DNS TXT propio (opcional)       → nombre soberano
alset-gen Go (Pi/OpenWrt)       → borde que controlas
Cloudflare Worker + DO          → borde global siempre on
CARGO mesh                      → tránsito entre bordes
Mind + announce                 → diálogo desde PrismaTec
```

## Límites honestos

- Free tier: límites de CPU/requests de Cloudflare.  
- El estado vive en **su** red (DO), no en tu router.  
- Para máxima soberanía: prioriza daemon propio + IPFS; CF es el ancla global.
