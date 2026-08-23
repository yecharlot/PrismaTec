# Persistencia Alset en Cloudflare (Durable Object)

## Objetivo

Sustituir o complementar Supabase con un store durable en el mismo Worker de la red Alset (`AlsetStoreDO`).

## API (Worker)

| Método | Ruta | Uso |
|--------|------|-----|
| GET | `/api/store/info` | Salud del DO |
| PUT | `/api/store/kv?key=` | body `{"data":"<base64>"}` |
| GET | `/api/store/kv?key=` | |
| DELETE | `/api/store/kv?key=` | |
| PUT | `/api/store/block?cid=` | body `{"data":"<base64>"}` |
| GET | `/api/store/block?cid=` | |
| POST | `/api/store/blocks` | `{"blocks":{"cid":"base64",...}}` |
| GET | `/api/store/blocks` | mapa completo |

Header opcional: `X-Alset-Store-Secret: <STORE_SECRET>`.

## Deploy Worker

```bash
cd cloudflare/alset-gen-worker
npx wrangler deploy
npx wrangler secret put STORE_SECRET   # opcional pero recomendado
```

## Nodo PrismaTec (env)

```text
ALSET_PERSIST=cloudflare
ALSET_CF_STORE_URL=https://alset-network.<cuenta>.workers.dev
ALSET_CF_STORE_SECRET=<mismo STORE_SECRET>
# o reutilizar:
ALSET_CLOUDFLARE_NETWORK=https://...
```

Prioridad en `NewFromEnv`: **Cloudflare → Supabase → local**.

## Pasos siguientes

- P2: activar en Render Elohim sin Supabase
- P3: genes como réplica de índice de episodios (memoria distribuida)
